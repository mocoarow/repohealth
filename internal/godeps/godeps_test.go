package godeps_test

import (
	"go/scanner"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/config"
	"github.com/mocoarow/repohealth/internal/godeps"
)

func file(src string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(src)}
}

// repo nests the modules svc-a and lib inside the root module, like a monorepo.
func repo() fstest.MapFS {
	return fstest.MapFS{
		"go.mod":                  file("module example.com/root\n"),
		"tools/t.go":              file("package tools\n\nimport (\n\t\"example.com/a/domain\"\n\t\"example.com/a/usecase\"\n)\n"),
		"svc-a/go.mod":            file("module example.com/a\n"),
		"svc-a/domain/d.go":       file("package domain\n\nimport \"example.com/lib/x\"\n"),
		"svc-a/usecase/u.go":      file("package usecase\n\nimport (\n\t\"fmt\"\n\t\"example.com/a/domain\"\n\t\"example.com/lib/x\"\n)\n"),
		"svc-a/usecase/v.go":      file("package usecase\n\nimport \"example.com/a/domain\"\n"),
		"svc-a/usecase/u_test.go": file("package usecase_test\n\nimport \"example.com/a/gateway\"\n"),
		"svc-a/gateway/g.go":      file("package gateway\n\nimport (\n\t\"example.com/a/domain\"\n\t\"example.com/a/usecase\"\n)\n"),
		"lib/go.mod":              file("module example.com/lib\n"),
		"lib/lib.go":              file("package lib\n"),
		"lib/x/x.go":              file("package x\n"),
	}
}

func files() []string {
	return []string{
		"go.mod", "tools/t.go", "svc-a/go.mod", "svc-a/domain/d.go", "svc-a/usecase/u.go", "svc-a/usecase/v.go",
		"svc-a/usecase/u_test.go", "svc-a/gateway/g.go", "lib/go.mod", "lib/lib.go", "lib/x/x.go",
	}
}

func testConfig() config.Config {
	return config.Config{
		Root:    "/repo",
		Version: 1,
		Areas: []config.Area{
			{Name: "a", Paths: []string{"svc-a"}},
			{Name: "lib", Paths: []string{"lib"}},
			{Name: "tools", Paths: []string{"tools"}},
		},
		Groups:         nil,
		Exclude:        nil,
		StablePackages: []string{"*/domain", "lib/x"},
	}
}

func load(t *testing.T) godeps.Graph {
	t.Helper()

	g, err := godeps.Load(repo(), testConfig(), files())
	require.NoError(t, err)

	return g
}

func Test_Load_shouldReturnRepositoryImportsOfEachPackage_whenModulesImportEachOther(t *testing.T) {
	t.Parallel()

	// given
	fsys, cfg := repo(), testConfig()

	// when
	got, err := godeps.Load(fsys, cfg, files())

	// then
	require.NoError(t, err)
	assert.Equal(t, godeps.Graph{Packages: []godeps.Package{
		{Dir: "lib", Path: "example.com/lib", Area: "lib", Imports: nil},
		{Dir: "lib/x", Path: "example.com/lib/x", Area: "lib", Imports: nil},
		{Dir: "svc-a/domain", Path: "example.com/a/domain", Area: "a", Imports: []string{"example.com/lib/x"}},
		{Dir: "svc-a/gateway", Path: "example.com/a/gateway", Area: "a", Imports: []string{"example.com/a/domain", "example.com/a/usecase"}},
		{Dir: "svc-a/usecase", Path: "example.com/a/usecase", Area: "a", Imports: []string{"example.com/a/domain", "example.com/lib/x"}},
		{Dir: "tools", Path: "example.com/root/tools", Area: "tools", Imports: []string{"example.com/a/domain", "example.com/a/usecase"}},
	}}, got)
}

func Test_Load_shouldCountImports_whenFileHasBuildConstraint(t *testing.T) {
	t.Parallel()

	// given
	fsys := repo()
	fsys["lib/x/gen.go"] = file("//go:build ignore\n\npackage main\n\nimport \"example.com/lib\"\n")

	// when
	got, err := godeps.Load(fsys, testConfig(), append(files(), "lib/x/gen.go"))

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"example.com/lib"}, importsOf(got, "lib/x"))
}

func importsOf(g godeps.Graph, dir string) []string {
	for _, p := range g.Packages {
		if p.Dir == dir {
			return p.Imports
		}
	}

	return nil
}

func Test_Load_shouldReturnErrNoModule_whenGoModHasNoModuleLine(t *testing.T) {
	t.Parallel()

	// given
	fsys := fstest.MapFS{"go.mod": file("module example.com/root\n"), "svc-a/go.mod": file("go 1.26\n"), "svc-a/a.go": file("package a\n")}

	// when
	_, err := godeps.Load(fsys, testConfig(), []string{"go.mod", "svc-a/go.mod", "svc-a/a.go"})

	// then
	require.ErrorIs(t, err, godeps.ErrNoModule)
}

func Test_Load_shouldSkipFile_whenFileIsNotCounted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		exclude []string
		areas   []config.Area
	}{
		{name: "excluded", exclude: []string{"svc-a/gateway/**"}, areas: testConfig().Areas},
		{name: "outside every area", exclude: nil, areas: []config.Area{{Name: "lib", Paths: []string{"lib"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			cfg := testConfig()
			cfg.Exclude = tt.exclude
			cfg.Areas = tt.areas

			// when
			got, err := godeps.Load(repo(), cfg, files())

			// then
			require.NoError(t, err)
			assert.NotContains(t, dirs(got), "svc-a/gateway")
		})
	}
}

func Test_Load_shouldReturnErrNoModule_whenNoGoModIsAbove(t *testing.T) {
	t.Parallel()

	// given
	fsys := fstest.MapFS{"svc-a/a.go": file("package a\n")}

	// when
	_, err := godeps.Load(fsys, testConfig(), []string{"svc-a/a.go"})

	// then
	require.ErrorIs(t, err, godeps.ErrNoModule)
}

func Test_Load_shouldReturnScannerError_whenFileDoesNotParse(t *testing.T) {
	t.Parallel()

	// given
	fsys := fstest.MapFS{"svc-a/go.mod": file("module example.com/a\n"), "svc-a/a.go": file("package a\nimport (")}

	// when
	_, err := godeps.Load(fsys, testConfig(), []string{"svc-a/go.mod", "svc-a/a.go"})

	// then
	var parseErr scanner.ErrorList
	require.ErrorAs(t, err, &parseErr)
}

func Test_Graph_AreaImports_shouldCountEachPackageImport_whenAreasDepend(t *testing.T) {
	t.Parallel()

	// given
	g := load(t)

	// when
	got := g.AreaImports()

	// then
	assert.Equal(t, []godeps.AreaImports{
		{From: "a", To: "lib", Imports: 2},
		{From: "tools", To: "a", Imports: 2},
	}, got)
}

func Test_Graph_AreaImports_shouldOrderByImportsThenAreas_whenSeveralPairsExist(t *testing.T) {
	t.Parallel()

	// given
	g := godeps.Graph{Packages: []godeps.Package{
		{Dir: "c", Path: "c", Area: "c", Imports: []string{"a"}},
		{Dir: "b", Path: "b", Area: "b", Imports: []string{"a"}},
		{Dir: "d1", Path: "d1", Area: "d", Imports: []string{"a"}},
		{Dir: "d2", Path: "d2", Area: "d", Imports: []string{"a"}},
		{Dir: "a", Path: "a", Area: "a", Imports: nil},
	}}

	// when
	got := g.AreaImports()

	// then
	assert.Equal(t, []godeps.AreaImports{
		{From: "d", To: "a", Imports: 2},
		{From: "b", To: "a", Imports: 1},
		{From: "c", To: "a", Imports: 1},
	}, got)
}

func Test_Graph_Stability_shouldReturnInstabilityOfStablePackages_whenPackagesAreImported(t *testing.T) {
	t.Parallel()

	// given
	g := load(t)

	// when
	got := g.Stability(testConfig())

	// then
	assert.Equal(t, []godeps.Stability{
		{Dir: "lib/x", Afferent: 2, Efferent: 0, Instability: 0},
		{Dir: "svc-a/domain", Afferent: 3, Efferent: 1, Instability: 0.25},
	}, got)
}

func Test_Graph_Stability_shouldReturnZero_whenPackageIsIsolated(t *testing.T) {
	t.Parallel()

	// given
	g := godeps.Graph{Packages: []godeps.Package{{Dir: "svc/domain", Path: "p", Area: "svc", Imports: nil}}}

	// when
	got := g.Stability(testConfig())

	// then
	assert.Equal(t, []godeps.Stability{{Dir: "svc/domain", Afferent: 0, Efferent: 0, Instability: 0}}, got)
}

func dirs(g godeps.Graph) []string {
	out := make([]string, 0, len(g.Packages))
	for _, p := range g.Packages {
		out = append(out, p.Dir)
	}

	return out
}
