package crossarea_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/config"
	"github.com/mocoarow/repohealth/internal/crossarea"
)

func testConfig() config.Config {
	return config.Config{
		Root:    "/repo",
		Version: 1,
		Areas: []config.Area{
			{Name: "auth", Paths: []string{"svc-auth"}},
			{Name: "question", Paths: []string{"svc-question"}},
			{Name: "web", Paths: []string{"svc-web"}},
		},
		Groups:  []config.Group{{Name: "backend", Areas: []string{"auth", "question"}}},
		Exclude: []string{"**/*_test.go"},
	}
}

func at(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err)

	return parsed
}

func change(t *testing.T, mergedAt string, files ...string) crossarea.Change {
	t.Helper()

	return crossarea.Change{MergedAt: at(t, mergedAt), Files: files}
}

func Test_Measure_shouldCountOnlyChangesTouchingAnArea_whenSomeTouchNone(t *testing.T) {
	t.Parallel()

	// given
	changes := []crossarea.Change{
		change(t, "2026-10-01T10:00:00+09:00", "svc-auth/a.go"),
		change(t, "2026-10-02T10:00:00+09:00", "svc-web/b.ts", "docs/readme.md"),
		change(t, "2026-10-03T10:00:00+09:00", "docs/readme.md"),
		change(t, "2026-10-04T10:00:00+09:00", "svc-auth/a_test.go"),
		change(t, "2026-10-05T10:00:00+09:00"),
	}

	// when
	got := crossarea.Measure(testConfig(), changes)

	// then
	assert.Equal(t, 2, got.Months[0].PRs)
}

func Test_Measure_shouldOmitMonth_whenNoChangeInItTouchesAnArea(t *testing.T) {
	t.Parallel()

	// given
	changes := []crossarea.Change{
		change(t, "2026-09-01T10:00:00+09:00", "docs/readme.md"),
		change(t, "2026-10-01T10:00:00+09:00", "svc-auth/a.go"),
	}

	// when
	got := crossarea.Measure(testConfig(), changes)

	// then
	assert.Equal(t, []string{"2026-10"}, monthNames(got.Months))
}

func monthNames(months []crossarea.Month) []string {
	names := make([]string, 0, len(months))
	for _, m := range months {
		names = append(names, m.Period)
	}

	return names
}

func Test_Measure_shouldCountCrossArea_whenChangeTouchesTwoAreas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files []string
		want  int
	}{
		{name: "two areas", files: []string{"svc-auth/a.go", "svc-web/b.ts"}, want: 1},
		{name: "two files in one area", files: []string{"svc-auth/a.go", "svc-auth/b.go"}, want: 0},
		{name: "second area only in excluded file", files: []string{"svc-auth/a.go", "svc-web/b_test.go"}, want: 0},
		{name: "second file outside every area", files: []string{"svc-auth/a.go", "docs/c.md"}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			changes := []crossarea.Change{change(t, "2026-10-01T10:00:00+09:00", tt.files...)}

			// when
			got := crossarea.Measure(testConfig(), changes)

			// then
			assert.Equal(t, tt.want, got.Months[0].CrossArea)
		})
	}
}

func Test_Measure_shouldCountGroupCrossArea_whenChangeTouchesTwoGroupAreas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		files []string
		want  int
	}{
		{name: "two group areas", files: []string{"svc-auth/a.go", "svc-question/b.go"}, want: 1},
		{name: "one group area and one other", files: []string{"svc-auth/a.go", "svc-web/b.ts"}, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			changes := []crossarea.Change{change(t, "2026-10-01T10:00:00+09:00", tt.files...)}

			// when
			got := crossarea.Measure(testConfig(), changes)

			// then
			assert.Equal(t, []crossarea.GroupCount{{Group: "backend", CrossArea: tt.want}}, got.Months[0].Groups)
		})
	}
}

func Test_Measure_shouldSplitByRecordedMonth_whenChangesSpanMonths(t *testing.T) {
	t.Parallel()

	// given
	changes := []crossarea.Change{
		change(t, "2026-11-01T00:30:00+09:00", "svc-auth/a.go", "svc-web/b.ts"),
		change(t, "2026-10-31T23:30:00+09:00", "svc-auth/a.go"),
		change(t, "2026-10-15T10:00:00+09:00", "svc-auth/a.go", "svc-question/b.go"),
	}

	// when
	got := crossarea.Measure(testConfig(), changes)

	// then
	assert.Equal(t, []crossarea.Month{
		{Period: "2026-10", PRs: 2, CrossArea: 1, Groups: []crossarea.GroupCount{{Group: "backend", CrossArea: 1}}},
		{Period: "2026-11", PRs: 1, CrossArea: 1, Groups: []crossarea.GroupCount{{Group: "backend", CrossArea: 0}}},
	}, got.Months)
}

func Test_Measure_shouldCountEveryPair_whenChangeTouchesThreeAreas(t *testing.T) {
	t.Parallel()

	// given
	changes := []crossarea.Change{
		change(t, "2026-10-01T10:00:00+09:00", "svc-web/c.ts", "svc-question/b.go", "svc-auth/a.go"),
		change(t, "2026-10-02T10:00:00+09:00", "svc-web/c.ts", "svc-question/b.go"),
	}

	// when
	got := crossarea.Measure(testConfig(), changes)

	// then
	assert.Equal(t, []crossarea.Pair{
		{A: "question", B: "web", PRs: 2},
		{A: "auth", B: "question", PRs: 1},
		{A: "auth", B: "web", PRs: 1},
	}, got.Pairs)
}

func Test_Measure_shouldReturnNoMonths_whenThereAreNoChanges(t *testing.T) {
	t.Parallel()

	// given
	var changes []crossarea.Change

	// when
	got := crossarea.Measure(testConfig(), changes)

	// then
	assert.Empty(t, got.Months)
}

func Test_Measure_shouldReturnNoPairs_whenNoChangeCrossesAreas(t *testing.T) {
	t.Parallel()

	// given
	changes := []crossarea.Change{change(t, "2026-10-01T10:00:00+09:00", "svc-auth/a.go")}

	// when
	got := crossarea.Measure(testConfig(), changes)

	// then
	assert.Empty(t, got.Pairs)
}
