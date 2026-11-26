package report

import (
	"fmt"

	"github.com/mocoarow/repohealth/internal/snapshot"
	"github.com/mocoarow/repohealth/internal/source"
)

type table struct {
	name   string
	header []string
	rows   [][]string
}

func writeLatest(dir string, d snapshot.Detail) error {
	functions, err := functionRows(d.Functions)
	if err != nil {
		return err
	}

	largest, err := largestRows(d)
	if err != nil {
		return err
	}

	tables := []table{
		crossAreaMonths(d),
		crossAreaPairs(d),
		hotspots(d),
		{name: "functions.csv", header: []string{"lang", "file", "line", "name", "cyclo", "cognit"}, rows: functions},
		areaImports(d),
		stability(d),
		coverageLayers(d),
		coverageReports(d),
		largest,
	}

	for _, t := range tables {
		if err := writeCSV(dir, t.name, t.header, t.rows); err != nil {
			return err
		}
	}

	return nil
}

func crossAreaMonths(d snapshot.Detail) table {
	header := append([]string{"period", "prs", "cross_area"}, crossAreaGroups(d, "cross_area_%s")...)

	rows := make([][]string, 0, len(d.CrossArea.Months))

	for _, m := range d.CrossArea.Months {
		counts := make([]string, 0, len(m.Groups))
		for _, g := range m.Groups {
			counts = append(counts, itoa(g.CrossArea))
		}

		rows = append(rows, append([]string{m.Period, itoa(m.PRs), itoa(m.CrossArea)}, counts...))
	}

	return table{name: "crossarea_months.csv", header: header, rows: rows}
}

// crossAreaGroups names the group columns with format; every month of a result carries the same groups in the same order.
func crossAreaGroups(d snapshot.Detail, format string) []string {
	if len(d.CrossArea.Months) == 0 {
		return nil
	}

	groups := make([]string, 0, len(d.CrossArea.Months[0].Groups))
	for _, g := range d.CrossArea.Months[0].Groups {
		groups = append(groups, fmt.Sprintf(format, g.Group))
	}

	return groups
}

func crossAreaPairs(d snapshot.Detail) table {
	rows := make([][]string, 0, len(d.CrossArea.Pairs))
	for _, p := range d.CrossArea.Pairs {
		rows = append(rows, []string{p.A, p.B, itoa(p.PRs)})
	}

	return table{name: "crossarea_pairs.csv", header: []string{"a", "b", "prs"}, rows: rows}
}

func hotspots(d snapshot.Detail) table {
	rows := make([][]string, 0, len(d.Hotspots))
	for _, h := range d.Hotspots {
		rows = append(rows, []string{h.Path, itoa(h.Churn), itoa(h.RecentChurn), itoa(h.SumCognit), itoa(h.MaxCognit), itoa(h.Score)})
	}

	header := []string{"path", "churn", "recent_churn", "sum_cognit", "max_cognit", "score"}

	return table{name: "hotspots.csv", header: header, rows: rows}
}

// functionRows leaves cyclo empty for TypeScript, which Biome does not measure.
func functionRows(funcs []snapshot.Function) ([][]string, error) {
	rows := make([][]string, 0, len(funcs))

	for _, f := range funcs {
		lang, err := f.Lang.MarshalText()
		if err != nil {
			return nil, fmt.Errorf("function %s in %s: %w", f.Name, f.File, err)
		}

		cyclo := ""
		if f.Lang != source.Script {
			cyclo = itoa(f.Cyclo)
		}

		rows = append(rows, []string{string(lang), f.File, itoa(f.Line), f.Name, cyclo, itoa(f.Cognit)})
	}

	return rows, nil
}

func areaImports(d snapshot.Detail) table {
	rows := make([][]string, 0, len(d.AreaImports))
	for _, i := range d.AreaImports {
		rows = append(rows, []string{i.From, i.To, itoa(i.Imports)})
	}

	return table{name: "area_imports.csv", header: []string{"from", "to", "imports"}, rows: rows}
}

func stability(d snapshot.Detail) table {
	rows := make([][]string, 0, len(d.Stability))
	for _, s := range d.Stability {
		rows = append(rows, []string{s.Dir, itoa(s.Afferent), itoa(s.Efferent), formatFloat(s.Instability, instabilDecimals)})
	}

	return table{name: "stability.csv", header: []string{"dir", "afferent", "efferent", "instability"}, rows: rows}
}

func coverageLayers(d snapshot.Detail) table {
	rows := make([][]string, 0, len(d.Coverage.Layers))
	for _, l := range d.Coverage.Layers {
		rows = append(rows, []string{l.Area, l.Layer, itoa(l.Covered), itoa(l.Lines)})
	}

	return table{name: "coverage_layers.csv", header: []string{"area", "layer", "covered", "lines"}, rows: rows}
}

func coverageReports(d snapshot.Detail) table {
	rows := make([][]string, 0, len(d.Coverage.Reports))
	for _, r := range d.Coverage.Reports {
		rows = append(rows, []string{r.File, formatTime(r.ModTime), itoa(r.Skipped)})
	}

	return table{name: "coverage_reports.csv", header: []string{"file", "mod_time", "skipped"}, rows: rows}
}

func largestRows(d snapshot.Detail) (table, error) {
	rows := make([][]string, 0, len(d.LargestFiles))

	for _, f := range d.LargestFiles {
		lang, err := f.Lang.MarshalText()
		if err != nil {
			return table{}, fmt.Errorf("largest file %s: %w", f.Path, err)
		}

		rows = append(rows, []string{f.Path, f.Area, string(lang), itoa(f.Lines)})
	}

	return table{name: "largest_files.csv", header: []string{"path", "area", "lang", "lines"}, rows: rows}, nil
}
