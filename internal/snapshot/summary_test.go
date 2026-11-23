package snapshot_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/complexity"
	"github.com/mocoarow/repohealth/internal/config"
	"github.com/mocoarow/repohealth/internal/coverage"
	"github.com/mocoarow/repohealth/internal/size"
	"github.com/mocoarow/repohealth/internal/snapshot"
	"github.com/mocoarow/repohealth/internal/source"
)

func fn(file string, line, cyclo, cognit int) complexity.Function {
	return complexity.Function{File: file, Line: line, Name: "f", Cyclo: cyclo, Cognit: cognit}
}

func header() snapshot.Header {
	return snapshot.Header{
		SchemaVersion: snapshot.SchemaVersion,
		TakenAt:       takenAt(),
		Rev:           "0123456789abcdef0123456789abcdef01234567",
		CommittedAt:   takenAt(),
		Tools:         snapshot.Tools{Repohealth: "v1.2.3", Biome: "2.0.0"},
	}
}

func Test_Summarize_shouldNotMixLanguages_whenAreaHasGoAndTS(t *testing.T) {
	t.Parallel()

	// given
	m := snapshot.Measurements{
		Go: []complexity.Function{fn("app/a.go", 1, 3, 4), fn("app/a.go", 9, 2, 1)},
		TS: []complexity.Function{fn("app/b.ts", 1, complexity.CycloNotMeasured, 30)},
	}

	// when
	got := snapshot.Summarize(appConfig("/repo"), header(), m)

	// then
	want := snapshot.GoComplexity{Functions: 2, CycloMax: 3, CognitSum: 5, CognitMax: 4, CognitOverLimit: 0}
	assert.Equal(t, want, got.Areas[0].Complexity.Go)
}

func Test_Summarize_shouldCountTSFunctions_whenCognitIsAtLeastTwo(t *testing.T) {
	t.Parallel()

	// given
	m := snapshot.Measurements{
		TS: []complexity.Function{fn("app/b.ts", 1, complexity.CycloNotMeasured, 2), fn("app/b.ts", 5, complexity.CycloNotMeasured, 6)},
	}

	// when
	got := snapshot.Summarize(appConfig("/repo"), header(), m)

	// then
	want := snapshot.TSComplexity{Functions: 2, CognitSum: 8, CognitMax: 6, CognitOverLimit: 0}
	assert.Equal(t, want, got.Areas[0].Complexity.TS)
}

func Test_Summarize_shouldCountOverLimit_whenCognitExceedsLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		lang   source.Lang
		cognit int
		wantGo int
		wantTS int
	}{
		{name: "go over limit", lang: source.Go, cognit: snapshot.CognitLimit + 1, wantGo: 1, wantTS: 0},
		{name: "ts over limit", lang: source.Script, cognit: snapshot.CognitLimit + 1, wantGo: 0, wantTS: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			funcs := []complexity.Function{fn("app/a", 1, complexity.CycloNotMeasured, tt.cognit)}
			m := snapshot.Measurements{Go: funcs}
			if tt.lang == source.Script {
				m = snapshot.Measurements{TS: funcs}
			}

			// when
			got := snapshot.Summarize(appConfig("/repo"), header(), m)

			// then
			c := got.Areas[0].Complexity
			assert.Equal(t, [2]int{tt.wantGo, tt.wantTS}, [2]int{c.Go.CognitOverLimit, c.TS.CognitOverLimit})
		})
	}
}

func Test_Summarize_shouldNotCountOverLimit_whenCognitEqualsLimit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		lang source.Lang
	}{
		{name: "go", lang: source.Go},
		{name: "ts", lang: source.Script},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			funcs := []complexity.Function{fn("app/a", 1, complexity.CycloNotMeasured, snapshot.CognitLimit)}
			m := snapshot.Measurements{Go: funcs}
			if tt.lang == source.Script {
				m = snapshot.Measurements{TS: funcs}
			}

			// when
			got := snapshot.Summarize(appConfig("/repo"), header(), m)

			// then
			c := got.Areas[0].Complexity
			assert.Equal(t, [2]int{0, 0}, [2]int{c.Go.CognitOverLimit, c.TS.CognitOverLimit})
		})
	}
}

func Test_Summarize_shouldCountOnlyOwnFunctions_whenOtherFilesAreOutsideArea(t *testing.T) {
	t.Parallel()

	// given
	cfg := config.Config{Root: "/repo", Version: 1, Areas: []config.Area{
		{Name: "app", Paths: []string{"app"}},
		{Name: "web", Paths: []string{"web"}},
	}}
	m := snapshot.Measurements{Go: []complexity.Function{
		fn("app/a.go", 1, 2, 3),
		fn("web/w.go", 1, 9, 9),
		fn("tools/t.go", 1, 8, 8),
	}}

	// when
	got := snapshot.Summarize(cfg, header(), m)

	// then
	want := snapshot.GoComplexity{Functions: 1, CycloMax: 2, CognitSum: 3, CognitMax: 3, CognitOverLimit: 0}
	assert.Equal(t, want, got.Areas[0].Complexity.Go)
}

func Test_Summarize_shouldListEveryArea_whenAreaHasNoFiles(t *testing.T) {
	t.Parallel()

	// given
	cfg := config.Config{Root: "/repo", Version: 1, Areas: []config.Area{
		{Name: "web", Paths: []string{"web"}},
		{Name: "app", Paths: []string{"app"}},
		{Name: "lib", Paths: []string{"lib"}},
	}}
	m := snapshot.Measurements{Go: []complexity.Function{fn("app/a.go", 1, 1, 1)}}

	// when
	got := snapshot.Summarize(cfg, header(), m)

	// then
	assert.Equal(t, []snapshot.Area{{Name: "web"}, {Name: "lib"}}, []snapshot.Area{got.Areas[0], got.Areas[2]})
}

func Test_Summarize_shouldSumCoverageByArea_whenAreaHasLayers(t *testing.T) {
	t.Parallel()

	// given
	m := snapshot.Measurements{Coverage: coverage.Result{Layers: []coverage.Layer{
		{Area: "app", Layer: "domain", Covered: 3, Lines: 4},
		{Area: "app", Layer: "handler", Covered: 1, Lines: 6},
	}}}

	// when
	got := snapshot.Summarize(appConfig("/repo"), header(), m)

	// then
	assert.Equal(t, snapshot.Lines{Covered: 4, Lines: 10}, got.Areas[0].Coverage)
}

func Test_Summarize_shouldSumSizeByLanguage_whenAreaHasFiles(t *testing.T) {
	t.Parallel()

	// given
	m := snapshot.Measurements{Sizes: []size.File{
		{Path: "app/a.go", Area: "app", Lang: source.Go, Test: false, Lines: 10},
		{Path: "app/a_test.go", Area: "app", Lang: source.Go, Test: true, Lines: 7},
		{Path: "app/b.ts", Area: "app", Lang: source.Script, Test: false, Lines: 5},
		{Path: "app/b.test.ts", Area: "app", Lang: source.Script, Test: true, Lines: 3},
	}}

	// when
	got := snapshot.Summarize(appConfig("/repo"), header(), m)

	// then
	want := snapshot.Size{Go: snapshot.LangSize{Lines: 10, TestLines: 7}, TS: snapshot.LangSize{Lines: 5, TestLines: 3}}
	assert.Equal(t, want, got.Areas[0].Size)
}

func Test_Summarize_shouldKeepCoverageReports_whenReportsWereRead(t *testing.T) {
	t.Parallel()

	// given
	reports := []coverage.Report{{File: "lcov.info", ModTime: takenAt(), Skipped: 2}}
	m := snapshot.Measurements{Coverage: coverage.Result{Reports: reports}}

	// when
	got := snapshot.Summarize(appConfig("/repo"), header(), m)

	// then
	assert.Equal(t, reports, got.CoverageReports)
}

func Test_Summarize_shouldWriteEmptyArrays_whenMeasurementsAreEmpty(t *testing.T) {
	t.Parallel()

	// given
	got := snapshot.Summarize(config.Config{Root: "/repo", Version: 1}, header(), snapshot.Measurements{})

	// when
	data, err := json.Marshal(got)

	// then
	require.NoError(t, err)
	assert.NotContains(t, string(data), "null")
}

func Test_Summarize_shouldFlattenHeader_whenMarshaled(t *testing.T) {
	t.Parallel()

	// given
	got := snapshot.Summarize(appConfig("/repo"), header(), snapshot.Measurements{})

	// when
	data, err := json.Marshal(got)

	// then
	require.NoError(t, err)
	assert.Contains(t, string(data), `{"schemaVersion":1,"takenAt":"2026-11-23T10:00:00Z","rev":"0123456789abcdef0123456789abcdef01234567",`)
}
