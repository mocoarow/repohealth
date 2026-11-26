package report

import (
	"fmt"
	"slices"

	"github.com/mocoarow/repohealth/internal/size"
	"github.com/mocoarow/repohealth/internal/snapshot"
	"github.com/mocoarow/repohealth/internal/source"
)

func areasHeader() []string {
	return []string{
		colArea,
		"Go lines", "Go test lines", "Go functions", "Go cyclo max", "Go cognit sum", "Go cognit max", "Go over limit",
		"TS lines", "TS test lines", "TS functions (cognit ≥ 2)", "TS cognit sum", "TS cognit max", "TS over limit",
		"Coverage",
	}
}

// mdAreas shows the snapshot of rev, the one the title names, and the last snapshot when no summary has rev.
func mdAreas(history []snapshot.Summary, rev string) [][]string {
	if len(history) == 0 {
		return nil
	}

	areas := history[len(history)-1].Areas
	if i := slices.IndexFunc(history, func(s snapshot.Summary) bool { return s.Rev == rev }); i >= 0 {
		areas = history[i].Areas
	}

	rows := make([][]string, 0, len(areas))

	for _, a := range areas {
		g, t := a.Complexity.Go, a.Complexity.TS
		rows = append(rows, []string{
			a.Name,
			itoa(a.Size.Go.Lines), itoa(a.Size.Go.TestLines), itoa(g.Functions), itoa(g.CycloMax), itoa(g.CognitSum), itoa(g.CognitMax), itoa(g.CognitOverLimit),
			itoa(a.Size.TS.Lines), itoa(a.Size.TS.TestLines), itoa(t.Functions), itoa(t.CognitSum), itoa(t.CognitMax), itoa(t.CognitOverLimit),
			coverageCell(a.Coverage.Covered, a.Coverage.Lines),
		})
	}

	return rows
}

// mdTrends shows the areas of the latest snapshot; an area an older snapshot lacks shows a dash.
func mdTrends(history []snapshot.Summary) []mdTable {
	recent := history[max(0, len(history)-MarkdownTrend):]

	var names []string
	if len(recent) > 0 {
		for _, a := range recent[len(recent)-1].Areas {
			names = append(names, a.Name)
		}
	}

	header := append([]string{"Committed", "Rev"}, names...)
	trend := func(heading string, value func(snapshot.Area) string) mdTable {
		rows := make([][]string, 0, len(recent))

		for _, s := range recent {
			row := make([]string, 0, len(header))
			row = append(row, formatTime(s.CommittedAt), s.Rev)

			for _, name := range names {
				row = append(row, areaValue(s, name, value))
			}

			rows = append(rows, row)
		}

		return mdTable{heading: heading, header: header, rows: rows}
	}

	return []mdTable{
		mdHeading("## Trends"),
		trend("### Coverage by area", func(a snapshot.Area) string { return coverageCell(a.Coverage.Covered, a.Coverage.Lines) }),
		trend("### Go cognitive complexity (sum)", func(a snapshot.Area) string { return itoa(a.Complexity.Go.CognitSum) }),
		trend("### TS cognitive complexity (sum)", func(a snapshot.Area) string { return itoa(a.Complexity.TS.CognitSum) }),
	}
}

func areaValue(s snapshot.Summary, name string, value func(snapshot.Area) string) string {
	for _, a := range s.Areas {
		if a.Name == name {
			return value(a)
		}
	}

	return dash
}

func mdCrossArea(d snapshot.Detail) []mdTable {
	months := crossAreaMonths(d)
	header := append([]string{"Month", "PRs", "Cross-area"}, crossAreaGroups(d, "Cross-area (%s)")...)

	pairs := d.CrossArea.Pairs[:min(len(d.CrossArea.Pairs), MarkdownTop)]
	pairRows := make([][]string, 0, len(pairs))

	for _, p := range pairs {
		pairRows = append(pairRows, []string{p.A + " + " + p.B, itoa(p.PRs)})
	}

	return []mdTable{
		mdHeading("## Cross-area changes"),
		{heading: "### Months", header: header, rows: months.rows},
		{heading: "### Pairs", header: []string{"Areas", "PRs"}, rows: pairRows},
	}
}

func mdDependencies(d snapshot.Detail) []mdTable {
	rows := make([][]string, 0, len(d.Stability))
	for _, s := range d.Stability {
		rows = append(rows, []string{code(s.Dir), itoa(s.Afferent), itoa(s.Efferent), formatFloat(s.Instability, instabilDecimals)})
	}

	return []mdTable{
		mdHeading("## Dependencies"),
		{heading: "### Area imports", header: []string{"From", "To", "Imports"}, rows: areaImports(d).rows},
		{heading: "### Stability", header: []string{"Package", "Afferent", "Efferent", "Instability"}, rows: rows},
	}
}

func mdHotspots(d snapshot.Detail) mdTable {
	files := d.Hotspots[:min(len(d.Hotspots), MarkdownTop)]
	rows := make([][]string, 0, len(files))

	for _, h := range files {
		rows = append(rows, []string{code(h.Path), itoa(h.Score), itoa(h.Churn), itoa(h.RecentChurn), itoa(h.SumCognit), itoa(h.MaxCognit)})
	}

	header := []string{colFile, "Score", "Churn", "Recent churn", "Cognit sum", "Cognit max"}

	return mdTable{heading: "## Hotspots", header: header, rows: rows}
}

// mdFunctions keeps the first MarkdownTop functions of each language, in the order the detail ranks them.
func mdFunctions(funcs []snapshot.Function) ([][]string, error) {
	shown := map[source.Lang]int{}

	var rows [][]string

	for _, f := range funcs {
		if shown[f.Lang] == MarkdownTop {
			continue
		}

		shown[f.Lang]++

		lang, err := langText(f.Lang)
		if err != nil {
			return nil, fmt.Errorf("function %s in %s: %w", f.Name, f.File, err)
		}

		cyclo := dash
		if f.Lang != source.Script {
			cyclo = itoa(f.Cyclo)
		}

		rows = append(rows, []string{lang, code(f.Name), code(f.File), itoa(f.Line), itoa(f.Cognit), cyclo})
	}

	return rows, nil
}

func mdCoverage(d snapshot.Detail) []mdTable {
	layers := make([][]string, 0, len(d.Coverage.Layers))
	for _, l := range d.Coverage.Layers {
		layers = append(layers, []string{l.Area, code(l.Layer), itoa(l.Covered), itoa(l.Lines), coverageCell(l.Covered, l.Lines)})
	}

	reports := make([][]string, 0, len(d.Coverage.Reports))
	for _, r := range d.Coverage.Reports {
		reports = append(reports, []string{code(r.File), formatTime(r.ModTime), itoa(r.Skipped)})
	}

	return []mdTable{
		mdHeading("## Coverage"),
		{heading: "### Layers", header: []string{colArea, "Layer", "Covered", "Lines", "Coverage"}, rows: layers},
		{heading: "### Reports", header: []string{colFile, "Modified", "Skipped"}, rows: reports},
	}
}

func mdLargest(files []size.File) ([][]string, error) {
	files = files[:min(len(files), MarkdownTop)]
	rows := make([][]string, 0, len(files))

	for _, f := range files {
		lang, err := langText(f.Lang)
		if err != nil {
			return nil, fmt.Errorf("largest file %s: %w", f.Path, err)
		}

		rows = append(rows, []string{code(f.Path), f.Area, lang, itoa(f.Lines)})
	}

	return rows, nil
}
