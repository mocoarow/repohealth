package report_test

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/report"
	"github.com/mocoarow/repohealth/internal/snapshot"
)

func readFile(t *testing.T, path string) string {
	t.Helper()

	b, err := os.ReadFile(path) //nolint:gosec // the path is a test file
	require.NoError(t, err)

	return string(b)
}

func Test_Write_shouldMatchGolden_whenInputIsFixed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
	}{
		{name: "history.csv"},
		{name: "crossarea_months.csv"},
		{name: "crossarea_pairs.csv"},
		{name: "hotspots.csv"},
		{name: "functions.csv"},
		{name: "area_imports.csv"},
		{name: "stability.csv"},
		{name: "coverage_layers.csv"},
		{name: "coverage_reports.csv"},
		{name: "largest_files.csv"},
	}

	dir := t.TempDir()
	require.NoError(t, report.Write(dir, []snapshot.Summary{fixedSummary(t)}, fixedDetail(t)))

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			want := readFile(t, filepath.Join("testdata", tt.name))

			// when
			got := readFile(t, filepath.Join(dir, tt.name))

			// then
			assert.Equal(t, want, got)
		})
	}
}

func Test_Write_shouldKeepLatestTakenAt_whenRevIsRepeated(t *testing.T) {
	t.Parallel()

	// the later time has the smaller string, so a string comparison would pick the other summary
	committed := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	earlier := headerOf("r1", time.Date(2026, 3, 4, 10, 0, 0, 0, time.FixedZone("", 9*60*60)), committed)
	later := headerOf("r1", time.Date(2026, 3, 4, 2, 0, 0, 0, time.UTC), committed)

	tests := []struct {
		name  string
		order func(t *testing.T) []snapshot.Summary
	}{
		{name: "newerFirst", order: func(t *testing.T) []snapshot.Summary {
			t.Helper()

			return []snapshot.Summary{skippedSummary(t, later, 2), skippedSummary(t, earlier, 1)}
		}},
		{name: "newerLast", order: func(t *testing.T) []snapshot.Summary {
			t.Helper()

			return []snapshot.Summary{skippedSummary(t, earlier, 1), skippedSummary(t, later, 2)}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			dir := t.TempDir()

			// when
			err := report.Write(dir, tt.order(t), fixedDetail(t))

			// then
			require.NoError(t, err)
			assert.Equal(t,
				"taken_at,committed_at,rev,metric,scope,value\n"+
					"2026-03-04T02:00:00Z,2026-03-01T00:00:00Z,r1,coverage_skipped,lcov.info,2\n",
				readFile(t, filepath.Join(dir, "history.csv")))
		})
	}
}

func Test_Write_shouldKeepLaterLine_whenRevIsRepeatedWithSameTakenAt(t *testing.T) {
	t.Parallel()

	// given
	h := headerOf("r1", time.Date(2026, 3, 4, 2, 0, 0, 0, time.UTC), time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))
	summaries := []snapshot.Summary{skippedSummary(t, h, 1), skippedSummary(t, h, 2)}
	dir := t.TempDir()

	// when
	err := report.Write(dir, summaries, fixedDetail(t))

	// then
	require.NoError(t, err)
	assert.Equal(t,
		"taken_at,committed_at,rev,metric,scope,value\n"+
			"2026-03-04T02:00:00Z,2026-03-01T00:00:00Z,r1,coverage_skipped,lcov.info,2\n",
		readFile(t, filepath.Join(dir, "history.csv")))
}

func Test_Write_shouldOrderByCommittedAt_whenOlderCommitIsTakenLater(t *testing.T) {
	t.Parallel()

	// given
	older := headerOf("old", time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))
	newer := headerOf("new", time.Date(2026, 3, 5, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC))
	summaries := []snapshot.Summary{skippedSummary(t, newer, 2), skippedSummary(t, older, 1)}
	dir := t.TempDir()

	// when
	err := report.Write(dir, summaries, fixedDetail(t))

	// then
	require.NoError(t, err)
	assert.Equal(t,
		"taken_at,committed_at,rev,metric,scope,value\n"+
			"2026-03-09T00:00:00Z,2026-03-01T00:00:00Z,old,coverage_skipped,lcov.info,1\n"+
			"2026-03-05T00:00:00Z,2026-03-03T00:00:00Z,new,coverage_skipped,lcov.info,2\n",
		readFile(t, filepath.Join(dir, "history.csv")))
}

func Test_Write_shouldOmitCoveragePct_whenAreaHasNoLines(t *testing.T) {
	t.Parallel()

	// given
	s := fixedSummary(t)
	s.Areas[0].Coverage = snapshot.Lines{Covered: 0, Lines: 0}
	dir := t.TempDir()

	// when
	err := report.Write(dir, []snapshot.Summary{s}, fixedDetail(t))

	// then
	require.NoError(t, err)

	assert.NotContains(t, readFile(t, filepath.Join(dir, "history.csv")), "coverage_pct")
}

func Test_Write_shouldWriteZeroCoverageLines_whenAreaHasNoLines(t *testing.T) {
	t.Parallel()

	// given
	s := fixedSummary(t)
	s.Areas[0].Coverage = snapshot.Lines{Covered: 0, Lines: 0}
	dir := t.TempDir()

	// when
	err := report.Write(dir, []snapshot.Summary{s}, fixedDetail(t))

	// then
	require.NoError(t, err)
	assert.Contains(t, readFile(t, filepath.Join(dir, "history.csv")), ",coverage_lines,api,0\n")
}

func Test_Write_shouldCreateDirectory_whenOutIsMissing(t *testing.T) {
	t.Parallel()

	// given
	dir := filepath.Join(t.TempDir(), "out", "nested")

	// when
	err := report.Write(dir, []snapshot.Summary{fixedSummary(t)}, fixedDetail(t))

	// then
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(dir, "history.csv"))
}

func Test_Write_shouldOverwrite_whenFileExists(t *testing.T) {
	t.Parallel()

	// given
	dir := t.TempDir()
	path := filepath.Join(dir, "hotspots.csv")
	require.NoError(t, os.WriteFile(path, []byte("stale"), 0o600))

	// when
	err := report.Write(dir, []snapshot.Summary{fixedSummary(t)}, fixedDetail(t))

	// then
	require.NoError(t, err)
	assert.Equal(t, readFile(t, filepath.Join("testdata", "hotspots.csv")), readFile(t, path))
}

func Test_Write_shouldReturnError_whenOutIsFile(t *testing.T) {
	t.Parallel()

	// given
	file := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(file, nil, 0o600))

	// when
	err := report.Write(file, []snapshot.Summary{fixedSummary(t)}, fixedDetail(t))

	// then
	require.ErrorIs(t, err, syscall.ENOTDIR)
}
