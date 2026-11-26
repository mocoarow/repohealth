package report_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/complexity"
	"github.com/mocoarow/repohealth/internal/coverage"
	"github.com/mocoarow/repohealth/internal/crossarea"
	"github.com/mocoarow/repohealth/internal/hotspot"
	"github.com/mocoarow/repohealth/internal/report"
	"github.com/mocoarow/repohealth/internal/size"
	"github.com/mocoarow/repohealth/internal/snapshot"
	"github.com/mocoarow/repohealth/internal/source"
)

func markdown(t *testing.T, summaries []snapshot.Summary, d snapshot.Detail) string {
	t.Helper()

	var b bytes.Buffer
	require.NoError(t, report.Markdown(&b, summaries, d))

	return b.String()
}

// section returns the lines from the heading line to the next heading of any level.
func section(t *testing.T, md, heading string) string {
	t.Helper()

	start := strings.Index(md, heading+"\n")
	require.GreaterOrEqual(t, start, 0, "heading %q not found", heading)

	rest := md[start+len(heading)+1:]
	if end := strings.Index(rest, "\n#"); end >= 0 {
		rest = rest[:end+1]
	}

	return heading + "\n" + rest
}

func Test_Markdown_shouldMatchGolden_whenInputIsFixed(t *testing.T) {
	t.Parallel()

	// given
	want := readFile(t, filepath.Join("testdata", "REPORT.md"))

	// when
	got := markdown(t, trendSummaries(t), fixedDetail(t))

	// then
	assert.Equal(t, want, got)
}

func Test_Markdown_shouldWriteDash_whenAreaIsMissingFromOlderSnapshot(t *testing.T) {
	t.Parallel()

	// given
	summaries := []snapshot.Summary{
		summaryOf(committedOn("r1", 1), areaOf("api", 1)),
		summaryOf(committedOn("r2", 2), areaOf("api", 2), areaOf("web", 3)),
	}

	// when
	got := section(t, markdown(t, summaries, fixedDetail(t)), "### Go cognitive complexity (sum)")

	// then
	assert.Contains(t, got, "| 2026-03-01T09:00:00+05:45 | r1 | 7 | - |\n")
}

func Test_Markdown_shouldOmitArea_whenAreaIsMissingFromLatestSnapshot(t *testing.T) {
	t.Parallel()

	// given
	summaries := []snapshot.Summary{
		summaryOf(committedOn("r1", 1), areaOf("api", 1), areaOf("gone", 9)),
		summaryOf(committedOn("r2", 2), areaOf("api", 2)),
	}

	// when
	got := section(t, markdown(t, summaries, fixedDetail(t)), "### Go cognitive complexity (sum)")

	// then
	assert.NotContains(t, got, "gone")
}

func Test_Markdown_shouldWriteDash_whenAreaHasNoCoverageLines(t *testing.T) {
	t.Parallel()

	// given
	a := areaOf("api", 1)
	a.Coverage = snapshot.Lines{Covered: 0, Lines: 0}
	summaries := []snapshot.Summary{summaryOf(committedOn("r1", 1), a)}

	// when
	got := section(t, markdown(t, summaries, fixedDetail(t)), "### Coverage by area")

	// then
	assert.Contains(t, got, "| 2026-03-01T09:00:00+05:45 | r1 | - |\n")
}

func Test_Markdown_shouldWriteDash_whenLayerHasNoLines(t *testing.T) {
	t.Parallel()

	// given
	d := fixedDetail(t)
	d.Coverage.Layers = []coverage.Layer{{Area: "api", Layer: "empty", Covered: 0, Lines: 0}}

	// when
	got := section(t, markdown(t, []snapshot.Summary{fixedSummary(t)}, d), "### Layers")

	// then
	assert.Contains(t, got, "| api | `empty` | 0 | 0 | - |\n")
}

func Test_Markdown_shouldWriteDash_whenFunctionIsTypeScript(t *testing.T) {
	t.Parallel()

	// given
	d := fixedDetail(t)

	// when
	got := section(t, markdown(t, []snapshot.Summary{fixedSummary(t)}, d), "## Functions")

	// then
	assert.Contains(t, got, "| ts | `render` | `web/app.ts` | 7 | 4 | - |\n")
}

func Test_Markdown_shouldKeepFirstFunctions_whenLanguageExceedsTop(t *testing.T) {
	t.Parallel()

	// given
	d := fixedDetail(t)
	d.Functions = nil
	for i := range report.MarkdownTop + 1 {
		d.Functions = append(d.Functions, snapshot.Function{
			Lang:     source.Go,
			Function: complexity.Function{File: "a.go", Line: i + 1, Name: fmt.Sprintf("f%d", i), Cyclo: 1, Cognit: 50 - i},
		})
	}

	// when
	got := section(t, markdown(t, []snapshot.Summary{fixedSummary(t)}, d), "## Functions")

	// then
	assert.Equal(t, report.MarkdownTop, strings.Count(got, "| go |"))
}

func Test_Markdown_shouldKeepLatestTakenAt_whenRevIsRepeated(t *testing.T) {
	t.Parallel()

	// given
	committed := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	earlier := summaryOf(headerOf("r1", time.Date(2026, 3, 4, 1, 0, 0, 0, time.UTC), committed), areaOf("api", 1))
	later := summaryOf(headerOf("r1", time.Date(2026, 3, 4, 2, 0, 0, 0, time.UTC), committed), areaOf("api", 2))

	// when
	got := section(t, markdown(t, []snapshot.Summary{later, earlier}, fixedDetail(t)), "### Go cognitive complexity (sum)")

	// then
	assert.Equal(t, "### Go cognitive complexity (sum)\n\n| Committed | Rev | api |\n| --- | --- | --- |\n| 2026-03-01T00:00:00Z | r1 | 14 |\n\n", got)
}

func Test_Markdown_shouldKeepLastSnapshots_whenHistoryExceedsTrendLimit(t *testing.T) {
	t.Parallel()

	// given
	summaries := make([]snapshot.Summary, 0, report.MarkdownTrend+1)
	for i := range report.MarkdownTrend + 1 {
		h := headerOf(fmt.Sprintf("rev%02d", i), time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 1, 1, i, 0, 0, 0, time.UTC))
		summaries = append(summaries, summaryOf(h, areaOf("api", 1)))
	}

	// when
	got := section(t, markdown(t, summaries, fixedDetail(t)), "### Go cognitive complexity (sum)")

	// then
	rows := strings.Split(strings.TrimSpace(got), "\n")[4:]
	assert.Contains(t, rows[0], "| rev01 |")
}

func Test_Markdown_shouldShowAreasOfTitleRev_whenLatestDetailIsNotLastCommitted(t *testing.T) {
	t.Parallel()

	// given
	summaries := []snapshot.Summary{
		summaryOf(committedOn("r1", 1), areaOf("api", 1)),
		summaryOf(committedOn("r2", 2), areaOf("api", 2)),
	}
	d := fixedDetail(t)
	d.Header = committedOn("r1", 1)

	// when
	got := section(t, markdown(t, summaries, d), "## Areas")

	// then
	assert.Contains(t, got, "| api | 100 |")
}

func Test_Markdown_shouldShowAreasOfLastSnapshot_whenTitleRevIsNotInHistory(t *testing.T) {
	t.Parallel()

	// given
	summaries := []snapshot.Summary{
		summaryOf(committedOn("r1", 1), areaOf("api", 1)),
		summaryOf(committedOn("r2", 2), areaOf("api", 2)),
	}

	// when
	got := section(t, markdown(t, summaries, fixedDetail(t)), "## Areas")

	// then
	assert.Contains(t, got, "| api | 200 |")
}

func Test_Markdown_shouldKeepTop_whenListExceedsTop(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		heading string
		fill    func(d *snapshot.Detail, i int)
	}{
		{name: "hotspots", heading: "## Hotspots", fill: func(d *snapshot.Detail, i int) {
			d.Hotspots = append(d.Hotspots, hotspot.File{Path: fmt.Sprintf("f%02d.go", i), Churn: 1, RecentChurn: 1, SumCognit: 1, MaxCognit: 1, Score: 1})
		}},
		{name: "pairs", heading: "### Pairs", fill: func(d *snapshot.Detail, i int) {
			d.CrossArea.Pairs = append(d.CrossArea.Pairs, crossarea.Pair{A: fmt.Sprintf("a%02d", i), B: "b", PRs: 1})
		}},
		{name: "largest files", heading: "## Largest files", fill: func(d *snapshot.Detail, i int) {
			d.LargestFiles = append(d.LargestFiles, size.File{Path: fmt.Sprintf("f%02d.go", i), Area: "api", Lang: source.Go, Test: false, Lines: 1})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			d := fixedDetail(t)
			d.Hotspots, d.CrossArea.Pairs, d.LargestFiles = nil, nil, nil
			for i := range report.MarkdownTop + 1 {
				tt.fill(&d, i)
			}

			// when
			got := section(t, markdown(t, []snapshot.Summary{fixedSummary(t)}, d), tt.heading)

			// then
			rows := strings.Split(strings.TrimSpace(got), "\n")[4:]
			assert.Len(t, rows, report.MarkdownTop)
		})
	}
}

func Test_WriteMarkdown_shouldReturnError_whenParentIsFile(t *testing.T) {
	t.Parallel()

	// given
	parent := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(parent, nil, 0o600))

	// when
	err := report.WriteMarkdown(filepath.Join(parent, "REPORT.md"), trendSummaries(t), fixedDetail(t))

	// then
	require.ErrorIs(t, err, syscall.ENOTDIR)
}

func Test_WriteMarkdown_shouldKeepFile_whenReportFails(t *testing.T) {
	t.Parallel()

	// given
	path := filepath.Join(t.TempDir(), "REPORT.md")
	require.NoError(t, os.WriteFile(path, []byte("old"), 0o600))

	d := fixedDetail(t)
	d.Functions[0].Lang = 0

	// when
	err := report.WriteMarkdown(path, trendSummaries(t), d)

	// then
	require.ErrorIs(t, err, source.ErrUnknownLang)
	assert.Equal(t, "old", readFile(t, path))
}

func Test_Markdown_shouldWriteNone_whenListIsEmpty(t *testing.T) {
	t.Parallel()

	// given
	d := fixedDetail(t)
	d.Hotspots = nil

	// when
	got := section(t, markdown(t, []snapshot.Summary{fixedSummary(t)}, d), "## Hotspots")

	// then
	assert.Equal(t, "## Hotspots\n\nNone.\n\n", got)
}

func Test_Markdown_shouldWriteNone_whenHistoryIsEmpty(t *testing.T) {
	t.Parallel()

	// given
	var summaries []snapshot.Summary

	// when
	got := section(t, markdown(t, summaries, fixedDetail(t)), "## Areas")

	// then
	assert.Equal(t, "## Areas\n\nNone.\n\n", got)
}

func Test_Markdown_shouldOmitBiome_whenBiomeVersionIsEmpty(t *testing.T) {
	t.Parallel()

	// given
	d := fixedDetail(t)
	d.Tools.Biome = ""

	// when
	got := markdown(t, []snapshot.Summary{fixedSummary(t)}, d)

	// then
	assert.Contains(t, got, "(repohealth 1.2.3).\n")
}

func Test_Markdown_shouldKeepCellText_whenCellHasMarkdownCharacters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "pipe", path: "a|b.go", want: "`a\\|b.go`"},
		{name: "underscores", path: "__x__.go", want: "`__x__.go`"},
		{name: "angle brackets", path: "<T>.go", want: "`<T>.go`"},
		{name: "backtick inside", path: "a`b.go", want: "``a`b.go``"},
		{name: "backtick at start", path: "`x.go", want: "`` `x.go ``"},
		{name: "newline", path: "a\nb.go", want: "`a b.go`"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			d := fixedDetail(t)
			d.Hotspots[0].Path = tt.path

			// when
			got := section(t, markdown(t, []snapshot.Summary{fixedSummary(t)}, d), "## Hotspots")

			// then
			assert.Contains(t, got, "| "+tt.want+" | 153 |")
		})
	}
}

func Test_Markdown_shouldWriteGroupColumns_whenMonthsHaveGroups(t *testing.T) {
	t.Parallel()

	// given
	d := fixedDetail(t)
	d.CrossArea.Months = []crossarea.Month{{Period: "2026-04", PRs: 6, CrossArea: 2, Groups: []crossarea.GroupCount{{Group: "backend", CrossArea: 1}}}}

	// when
	got := section(t, markdown(t, []snapshot.Summary{fixedSummary(t)}, d), "### Months")

	// then
	assert.Equal(t, "### Months\n\n| Month | PRs | Cross-area | Cross-area (backend) |\n| --- | --- | --- | --- |\n| 2026-04 | 6 | 2 | 1 |\n\n", got)
}

func Test_WriteMarkdown_shouldCreateDirectory_whenParentIsMissing(t *testing.T) {
	t.Parallel()

	// given
	path := filepath.Join(t.TempDir(), "out", "nested", "REPORT.md")

	// when
	err := report.WriteMarkdown(path, trendSummaries(t), fixedDetail(t))

	// then
	require.NoError(t, err)
	assert.Equal(t, readFile(t, filepath.Join("testdata", "REPORT.md")), readFile(t, path))
}
