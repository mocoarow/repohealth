package complexity_test

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/complexity"
	"github.com/mocoarow/repohealth/internal/config"
)

func scriptConfig(t *testing.T) config.Config {
	t.Helper()

	root, err := filepath.Abs(filepath.Join("testdata", "ts"))
	require.NoError(t, err)

	return config.Config{
		Root:    root,
		Version: 1,
		Areas:   []config.Area{{Name: "web", Paths: []string{"web"}}},
		Groups:  nil,
		Exclude: []string{"web/gen/**"},
	}
}

func Test_TypeScript_shouldReturnCognitiveComplexity_whenFunctionsBranch(t *testing.T) {
	t.Parallel()

	// given
	cfg := scriptConfig(t)

	// when
	got, err := complexity.TypeScript(t.Context(), cfg, []string{"web/app.ts"})

	// then
	require.NoError(t, err)
	assert.Equal(t, []complexity.Function{
		{File: "web/app.ts", Line: 1, Name: "nested", Cyclo: complexity.CycloNotMeasured, Cognit: 6},
		{File: "web/app.ts", Line: 13, Name: "(anonymous)", Cyclo: complexity.CycloNotMeasured, Cognit: 2},
		{File: "web/app.ts", Line: 23, Name: "afterWide", Cyclo: complexity.CycloNotMeasured, Cognit: 2},
	}, got)
}

func Test_TypeScript_shouldMeasureEveryArea_whenFilesSpanAreas(t *testing.T) {
	t.Parallel()

	// given
	cfg := scriptConfig(t)
	cfg.Areas = append(cfg.Areas, config.Area{Name: "other", Paths: []string{"other"}})

	// when
	got, err := complexity.TypeScript(t.Context(), cfg, []string{"web/app.ts", "other/other.ts"})

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"other/other.ts", "web/app.ts", "web/app.ts", "web/app.ts"}, fileNames(got))
}

func fileNames(funcs []complexity.Function) []string {
	out := make([]string, 0, len(funcs))
	for _, f := range funcs {
		out = append(out, f.File)
	}

	return out
}

func Test_TypeScript_shouldSkipFile_whenFileIsNotCounted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
	}{
		{name: "test file", file: "web/app.test.ts"},
		{name: "declaration file", file: "web/types.d.ts"},
		{name: "excluded file", file: "web/gen/generated.ts"},
		{name: "outside every area", file: "other/other.ts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			cfg := scriptConfig(t)

			// when
			got, err := complexity.TypeScript(t.Context(), cfg, []string{tt.file})

			// then
			require.NoError(t, err)
			assert.Empty(t, got)
		})
	}
}

func Test_TypeScript_shouldOmitFunction_whenBiomeIgnoreSuppressesIt(t *testing.T) {
	t.Parallel()

	// given
	cfg := scriptConfig(t)

	// when
	got, err := complexity.TypeScript(t.Context(), cfg, []string{"web/ignored.ts"})

	// then
	require.NoError(t, err)
	assert.Empty(t, got)
}

func Test_TypeScript_shouldReturnErrNotExist_whenAreaDirectoryIsMissing(t *testing.T) {
	t.Parallel()

	// given
	cfg := scriptConfig(t)
	cfg.Root = t.TempDir()

	// when
	_, err := complexity.TypeScript(t.Context(), cfg, []string{"web/app.ts"})

	// then
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func Test_ParseBiome_shouldReturnErrUnexpectedBiomeOutput_whenOutputIsNotJSON(t *testing.T) {
	t.Parallel()

	// given
	out := []byte("Biome failed")

	// when
	err := complexity.ParseBiome(out)

	// then
	require.ErrorIs(t, err, complexity.ErrUnexpectedBiomeOutput)
}

func Test_CognitOf_shouldReturnErrUnexpectedBiomeOutput_whenMessageHasNoComplexity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message string
	}{
		{name: "other wording", message: "Excessive nesting detected."},
		{name: "number too large", message: "Excessive complexity of 99999999999999999999 detected (max: 1)."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			message := tt.message

			// when
			err := complexity.CognitOf(message)

			// then
			require.ErrorIs(t, err, complexity.ErrUnexpectedBiomeOutput)
		})
	}
}

func Test_NameAt_shouldReturnAnonymous_whenSpanIsNotAnIdentifier(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                   string
		startLine, startColumn int
		endLine, endColumn     int
	}{
		{name: "arrow", startLine: 1, startColumn: 14, endLine: 1, endColumn: 16},
		{name: "function keyword", startLine: 2, startColumn: 11, endLine: 2, endColumn: 19},
		{name: "line past end", startLine: 4, startColumn: 1, endLine: 4, endColumn: 2},
		{name: "zero line", startLine: 0, startColumn: 1, endLine: 0, endColumn: 2},
		{name: "spans lines", startLine: 1, startColumn: 7, endLine: 2, endColumn: 2},
		{name: "zero column", startLine: 1, startColumn: 0, endLine: 1, endColumn: 2},
		{name: "column past end", startLine: 1, startColumn: 9, endLine: 1, endColumn: 40},
		{name: "empty span", startLine: 1, startColumn: 5, endLine: 1, endColumn: 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			src := "const f = () => 1;\nconst g = function(a) { return a; };\n"
			sources := fstest.MapFS{"a.ts": &fstest.MapFile{Data: []byte(src)}}

			// when
			got, err := complexity.NameAt(sources, "a.ts", tt.startLine, tt.startColumn, tt.endLine, tt.endColumn)

			// then
			require.NoError(t, err)
			assert.Equal(t, "(anonymous)", got)
		})
	}
}

func Test_NameAt_shouldReturnErrNotExist_whenFileIsMissing(t *testing.T) {
	t.Parallel()

	// given
	sources := fstest.MapFS{}

	// when
	_, err := complexity.NameAt(sources, "a.ts", 1, 1, 1, 2)

	// then
	require.ErrorIs(t, err, fs.ErrNotExist)
}

const batchFiles = 201

const branchingSource = "export function f(a: number): number {\n  if (a > 0) {\n    if (a > 1) {\n      return 1;\n    }\n  }\n  return 0;\n}\n"

func gitRepoWithWebArea(t *testing.T, files map[string]string) config.Config {
	t.Helper()

	root := t.TempDir()

	initCmd := exec.CommandContext(t.Context(), "git", "init", "-q")
	initCmd.Dir = root
	out, err := initCmd.CombinedOutput()
	require.NoError(t, err, string(out))

	for name, body := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o750))
		require.NoError(t, os.WriteFile(full, []byte(body), 0o600))
	}

	return config.Config{
		Root:    root,
		Version: 1,
		Areas:   []config.Area{{Name: "web", Paths: []string{"web"}}},
		Groups:  nil,
		Exclude: nil,
	}
}

func Test_TypeScript_shouldMeasureFile_whenGitignoreIgnoresIt(t *testing.T) {
	t.Parallel()

	// given
	cfg := gitRepoWithWebArea(t, map[string]string{
		"web/.gitignore": "/build/\n",
		"web/build/x.ts": branchingSource,
	})

	// when
	got, err := complexity.TypeScript(t.Context(), cfg, []string{"web/build/x.ts"})

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"web/build/x.ts"}, fileNames(got))
}

func Test_TypeScript_shouldMeasureEveryFile_whenFilesExceedBatch(t *testing.T) {
	t.Parallel()

	// given
	bodies := map[string]string{}
	files := make([]string, 0, batchFiles)

	for i := range batchFiles {
		name := fmt.Sprintf("web/f%03d.ts", i)
		bodies[name] = branchingSource
		files = append(files, name)
	}

	cfg := gitRepoWithWebArea(t, bodies)

	// when
	got, err := complexity.TypeScript(t.Context(), cfg, files)

	// then
	require.NoError(t, err)
	assert.Len(t, got, batchFiles)
}

func Test_TypeScript_shouldMeasureFile_whenNameHasGlobOrDollarCharacters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
	}{
		{name: "square brackets", file: "web/app/[id]/page.ts"},
		{name: "dollar sign", file: "web/routes/workbooks.$workbookId.ts"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			cfg := gitRepoWithWebArea(t, map[string]string{tt.file: branchingSource})

			// when
			got, err := complexity.TypeScript(t.Context(), cfg, []string{tt.file})

			// then
			require.NoError(t, err)
			assert.Equal(t, []string{tt.file}, fileNames(got))
		})
	}
}

func Test_TypeScript_shouldMeasureFile_whenAreaPathIsRoot(t *testing.T) {
	t.Parallel()

	// given
	cfg := gitRepoWithWebArea(t, map[string]string{"web/app.ts": branchingSource})
	cfg.Areas = []config.Area{{Name: "all", Paths: []string{"."}}}

	// when
	got, err := complexity.TypeScript(t.Context(), cfg, []string{"web/app.ts"})

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"web/app.ts"}, fileNames(got))
}

func Test_TypeScript_shouldReturnErrBiomeInternal_whenTargetFileIsMissing(t *testing.T) {
	t.Parallel()

	// given
	cfg := gitRepoWithWebArea(t, map[string]string{"web/app.ts": branchingSource})

	// when
	_, err := complexity.TypeScript(t.Context(), cfg, []string{"web/missing.ts"})

	// then
	require.ErrorIs(t, err, complexity.ErrBiomeInternal)
}

func Test_TypeScript_shouldReturnErrBiomeInternalWithMessage_whenTargetFileIsMissing(t *testing.T) {
	t.Parallel()

	// given
	cfg := gitRepoWithWebArea(t, map[string]string{"web/app.ts": branchingSource})

	// when
	_, err := complexity.TypeScript(t.Context(), cfg, []string{"web/missing.ts"})

	// then
	require.ErrorContains(t, err, "No such file or directory")
}

func Test_TypeScript_shouldReturnErrBiomeParse_whenFileHasSyntaxError(t *testing.T) {
	t.Parallel()

	// given
	cfg := gitRepoWithWebArea(t, map[string]string{"web/bad.ts": "export const x = (;\n"})

	// when
	_, err := complexity.TypeScript(t.Context(), cfg, []string{"web/bad.ts"})

	// then
	require.ErrorIs(t, err, complexity.ErrBiomeParse)
}

func Test_TypeScript_shouldNameFileAndMessage_whenFileHasSyntaxError(t *testing.T) {
	t.Parallel()

	// given
	cfg := gitRepoWithWebArea(t, map[string]string{"web/bad.ts": "export const x = (;\n"})

	// when
	_, err := complexity.TypeScript(t.Context(), cfg, []string{"web/bad.ts"})

	// then
	require.ErrorContains(t, err, "bad.ts: expected `)` but instead found `;`")
}

func Test_FunctionsOf_shouldReturnErrUnexpectedBiomeOutput_whenDiagnosticPathWasNotRequested(t *testing.T) {
	t.Parallel()

	// given
	sources := fstest.MapFS{"web/a.ts": {Data: []byte(branchingSource)}}

	// when
	err := complexity.FunctionsOf(sources, "web", []string{"web/a.ts"}, "other.ts")

	// then
	require.ErrorIs(t, err, complexity.ErrUnexpectedBiomeOutput)
}

func Test_FunctionsOf_shouldNamePath_whenDiagnosticPathWasNotRequested(t *testing.T) {
	t.Parallel()

	// given
	sources := fstest.MapFS{"web/a.ts": {Data: []byte(branchingSource)}}

	// when
	err := complexity.FunctionsOf(sources, "web", []string{"web/a.ts"}, "other.ts")

	// then
	require.ErrorContains(t, err, "web/other.ts")
}

func Test_TypeScript_shouldMeasureFile_whenFileExceedsDefaultMaxSize(t *testing.T) {
	t.Parallel()

	// given
	const overBiomeDefaultBytes = 1100 * 1024

	cfg := gitRepoWithWebArea(t, map[string]string{
		"web/big.ts": branchingSource + "// " + strings.Repeat("x", overBiomeDefaultBytes) + "\n",
	})

	// when
	got, err := complexity.TypeScript(t.Context(), cfg, []string{"web/big.ts"})

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"web/big.ts"}, fileNames(got))
}
