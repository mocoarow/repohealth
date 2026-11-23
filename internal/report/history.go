package report

import (
	"cmp"
	"slices"

	"github.com/mocoarow/repohealth/internal/snapshot"
)

const historyName = "history.csv"

type metricRow struct {
	metric string
	scope  string
	value  string
}

func writeHistory(dir string, summaries []snapshot.Summary) error {
	header := []string{"taken_at", "committed_at", "rev", "metric", "scope", "value"}

	var rows [][]string

	for _, s := range sortedSummaries(latestPerRev(summaries)) {
		for _, r := range metricRows(s) {
			rows = append(rows, []string{formatTime(s.TakenAt), formatTime(s.CommittedAt), s.Rev, r.metric, r.scope, r.value})
		}
	}

	return writeCSV(dir, historyName, header, rows)
}

// latestPerRev keeps one summary per rev, the one taken last (the later line when taken at the same time), so a snapshot taken twice is not counted twice.
func latestPerRev(summaries []snapshot.Summary) []snapshot.Summary {
	byRev := make(map[string]snapshot.Summary, len(summaries))

	for _, s := range summaries {
		if prev, ok := byRev[s.Rev]; !ok || !s.TakenAt.Before(prev.TakenAt) {
			byRev[s.Rev] = s
		}
	}

	out := make([]snapshot.Summary, 0, len(byRev))
	for _, s := range byRev {
		out = append(out, s)
	}

	return out
}

func sortedSummaries(summaries []snapshot.Summary) []snapshot.Summary {
	slices.SortFunc(summaries, func(a, b snapshot.Summary) int {
		return cmp.Or(a.CommittedAt.Compare(b.CommittedAt), a.TakenAt.Compare(b.TakenAt), cmp.Compare(a.Rev, b.Rev))
	})

	return summaries
}

func metricRows(s snapshot.Summary) []metricRow {
	rows := make([]metricRow, 0, len(s.Areas)+len(s.AreaImports)+len(s.Stability)+len(s.CoverageReports))

	for _, a := range s.Areas {
		rows = append(rows, areaRows(a)...)
	}

	for _, i := range s.AreaImports {
		rows = append(rows, metricRow{"area_imports", i.From + ">" + i.To, itoa(i.Imports)})
	}

	for _, st := range s.Stability {
		rows = append(rows,
			metricRow{"afferent", st.Dir, itoa(st.Afferent)},
			metricRow{"efferent", st.Dir, itoa(st.Efferent)},
			metricRow{"instability", st.Dir, formatFloat(st.Instability, instabilDecimals)},
		)
	}

	for _, r := range s.CoverageReports {
		rows = append(rows, metricRow{"coverage_skipped", r.File, itoa(r.Skipped)})
	}

	slices.SortFunc(rows, func(a, b metricRow) int {
		return cmp.Or(cmp.Compare(a.metric, b.metric), cmp.Compare(a.scope, b.scope))
	})

	return rows
}

func areaRows(a snapshot.Area) []metricRow {
	n, c := a.Name, a.Complexity
	rows := []metricRow{
		{"coverage_covered", n, itoa(a.Coverage.Covered)},
		{"coverage_lines", n, itoa(a.Coverage.Lines)},
		{"go_lines", n, itoa(a.Size.Go.Lines)},
		{"go_test_lines", n, itoa(a.Size.Go.TestLines)},
		{"ts_lines", n, itoa(a.Size.TS.Lines)},
		{"ts_test_lines", n, itoa(a.Size.TS.TestLines)},
		{"go_functions", n, itoa(c.Go.Functions)},
		{"go_cyclo_max", n, itoa(c.Go.CycloMax)},
		{"go_cognit_sum", n, itoa(c.Go.CognitSum)},
		{"go_cognit_max", n, itoa(c.Go.CognitMax)},
		{"go_cognit_over_limit", n, itoa(c.Go.CognitOverLimit)},
		{"ts_cognit_functions", n, itoa(c.TS.Functions)},
		{"ts_cognit_sum", n, itoa(c.TS.CognitSum)},
		{"ts_cognit_max", n, itoa(c.TS.CognitMax)},
		{"ts_cognit_over_limit", n, itoa(c.TS.CognitOverLimit)},
	}

	if a.Coverage.Lines > 0 {
		pct := float64(a.Coverage.Covered) * percentScale / float64(a.Coverage.Lines)
		rows = append(rows, metricRow{"coverage_pct", n, formatFloat(pct, pctDecimals)})
	}

	return rows
}
