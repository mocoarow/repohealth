package snapshot_test

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/complexity"
	"github.com/mocoarow/repohealth/internal/gitlog"
	"github.com/mocoarow/repohealth/internal/godeps"
	"github.com/mocoarow/repohealth/internal/hotspot"
	"github.com/mocoarow/repohealth/internal/size"
	"github.com/mocoarow/repohealth/internal/snapshot"
	"github.com/mocoarow/repohealth/internal/source"
)

const overLimit = snapshot.DetailLimit + 5

func goFile(path string, test bool, lines int) size.File {
	return size.File{Path: path, Area: "app", Lang: source.Go, Test: test, Lines: lines}
}

func Test_Detailed_shouldKeepTopFunctions_whenFunctionsExceedLimit(t *testing.T) {
	t.Parallel()

	// given
	var funcs []complexity.Function
	for i := 1; i <= overLimit; i++ {
		funcs = append(funcs, fn("app/a.go", i, 1, i))
	}

	// when
	got := snapshot.Detailed(appConfig("/repo"), header(), snapshot.Measurements{Go: funcs})

	// then
	var want []snapshot.Function
	for i := overLimit; i > overLimit-snapshot.DetailLimit; i-- {
		want = append(want, snapshot.Function{Lang: source.Go, Function: fn("app/a.go", i, 1, i)})
	}

	assert.Equal(t, want, got.Functions)
}

func Test_Detailed_shouldKeepTopFunctionsPerLanguage_whenBothLanguagesExceedLimit(t *testing.T) {
	t.Parallel()

	// given
	var goFuncs, tsFuncs []complexity.Function
	for i := 1; i <= overLimit; i++ {
		goFuncs = append(goFuncs, fn("app/a.go", i, 1, i))
		tsFuncs = append(tsFuncs, fn("app/b.ts", i, complexity.CycloNotMeasured, i+100))
	}

	// when
	got := snapshot.Detailed(appConfig("/repo"), header(), snapshot.Measurements{Go: goFuncs, TS: tsFuncs})

	// then
	counts := map[source.Lang]int{}
	for _, f := range got.Functions {
		counts[f.Lang]++
	}

	assert.Equal(t, map[source.Lang]int{source.Go: snapshot.DetailLimit, source.Script: snapshot.DetailLimit}, counts)
}

func Test_Detailed_shouldOrderTiesByFileAndLine_whenCognitIsEqual(t *testing.T) {
	t.Parallel()

	// given
	funcs := []complexity.Function{fn("app/b.go", 1, 1, 5), fn("app/a.go", 9, 1, 5), fn("app/a.go", 2, 1, 5)}

	// when
	got := snapshot.Detailed(appConfig("/repo"), header(), snapshot.Measurements{Go: funcs})

	// then
	want := []snapshot.Function{
		{Lang: source.Go, Function: fn("app/a.go", 2, 1, 5)},
		{Lang: source.Go, Function: fn("app/a.go", 9, 1, 5)},
		{Lang: source.Go, Function: fn("app/b.go", 1, 1, 5)},
	}
	assert.Equal(t, want, got.Functions)
}

func Test_Detailed_shouldKeepLargestProductionFiles_whenFilesIncludeTests(t *testing.T) {
	t.Parallel()

	// given
	sizes := []size.File{
		goFile("app/a_test.go", true, 500),
		goFile("app/b.go", false, 10),
		goFile("app/c.go", false, 40),
		goFile("app/a.go", false, 10),
	}

	// when
	got := snapshot.Detailed(appConfig("/repo"), header(), snapshot.Measurements{Sizes: sizes})

	// then
	want := []size.File{goFile("app/c.go", false, 40), goFile("app/a.go", false, 10), goFile("app/b.go", false, 10)}
	assert.Equal(t, want, got.LargestFiles)
}

func Test_Detailed_shouldKeepTopLargestFiles_whenFilesExceedLimit(t *testing.T) {
	t.Parallel()

	// given
	var sizes []size.File
	for i := 1; i <= overLimit; i++ {
		sizes = append(sizes, goFile(fmt.Sprintf("app/f%02d.go", i), false, i))
	}

	// when
	got := snapshot.Detailed(appConfig("/repo"), header(), snapshot.Measurements{Sizes: sizes})

	// then
	assert.Len(t, got.LargestFiles, snapshot.DetailLimit)
}

func Test_Detailed_shouldKeepTopHotspots_whenRankExceedsLimit(t *testing.T) {
	t.Parallel()

	// given
	var funcs []complexity.Function

	churn := map[string]gitlog.FileChurn{}

	for i := 1; i <= overLimit; i++ {
		file := fmt.Sprintf("app/f%02d.go", i)
		funcs = append(funcs, fn(file, 1, 1, 2))
		churn[file] = gitlog.FileChurn{Total: i, Recent: 0}
	}

	// when
	got := snapshot.Detailed(appConfig("/repo"), header(), snapshot.Measurements{Go: funcs, Churn: churn})

	// then
	assert.Equal(t, hotspot.Rank(funcs, churn)[:snapshot.DetailLimit], got.Hotspots)
}

func Test_Detailed_shouldWriteEmptyArrays_whenMeasurementsAreEmpty(t *testing.T) {
	t.Parallel()

	// given
	got := snapshot.Detailed(appConfig("/repo"), header(), snapshot.Measurements{})

	// when
	data, err := json.Marshal(got)

	// then
	require.NoError(t, err)
	assert.NotContains(t, string(data), "null")
}

func Test_Detailed_shouldFlattenFunction_whenMarshaled(t *testing.T) {
	t.Parallel()

	// given
	got := snapshot.Detailed(appConfig("/repo"), header(), snapshot.Measurements{Go: []complexity.Function{fn("app/a.go", 3, 2, 1)}})

	// when
	data, err := json.Marshal(got.Functions)

	// then
	require.NoError(t, err)
	assert.JSONEq(t, `[{"lang":"go","file":"app/a.go","line":3,"name":"f","cyclo":2,"cognit":1}]`, string(data))
}

func Test_Detailed_shouldReportStability_whenPackageIsStable(t *testing.T) {
	t.Parallel()

	// given
	cfg := appConfig("/repo")
	cfg.StablePackages = []string{"app/domain"}
	graph := godeps.Graph{Packages: []godeps.Package{
		{Dir: "app/domain", Path: "example.com/r/app/domain", Area: "app", Imports: nil},
		{Dir: "app/handler", Path: "example.com/r/app/handler", Area: "app", Imports: []string{"example.com/r/app/domain"}},
	}}

	// when
	got := snapshot.Detailed(cfg, header(), snapshot.Measurements{Graph: graph})

	// then
	assert.Equal(t, []godeps.Stability{{Dir: "app/domain", Afferent: 1, Efferent: 0, Instability: 0}}, got.Stability)
}
