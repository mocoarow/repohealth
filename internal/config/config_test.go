package config_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/config"
)

func writeRepo(t *testing.T, yml string, dirs ...string) string {
	t.Helper()

	root := t.TempDir()
	for _, d := range dirs {
		require.NoError(t, os.MkdirAll(filepath.Join(root, d), 0o750))
	}

	path := filepath.Join(root, config.FileName)
	require.NoError(t, os.WriteFile(path, []byte(yml), 0o600))

	return path
}

const validYAML = `
version: 1
areas:
  - name: auth
    paths: [svc-auth]
  - name: web
    paths: [svc-web, ./shared/ui/]
exclude:
  - "**/*_mock.go"
`

func Test_Load_shouldReturnCleanedAreas_whenConfigIsValid(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, validYAML, "svc-auth", "svc-web", "shared/ui")

	// when
	cfg, err := config.Load(path)

	// then
	require.NoError(t, err)
	assert.Equal(t, []config.Area{
		{Name: "auth", Paths: []string{"svc-auth"}},
		{Name: "web", Paths: []string{"svc-web", "shared/ui"}},
	}, cfg.Areas)
}

func Test_Load_shouldReturnExclude_whenConfigIsValid(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, validYAML, "svc-auth", "svc-web", "shared/ui")

	// when
	cfg, err := config.Load(path)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"**/*_mock.go"}, cfg.Exclude)
}

func Test_Load_shouldSetRootToConfigDirectory_whenConfigIsValid(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, validYAML, "svc-auth", "svc-web", "shared/ui")

	// when
	cfg, err := config.Load(path)

	// then
	require.NoError(t, err)
	assert.Equal(t, filepath.Dir(path), cfg.Root)
}

func Test_Load_shouldReturnErrNotExist_whenFileIsMissing(t *testing.T) {
	t.Parallel()

	// given
	path := filepath.Join(t.TempDir(), config.FileName)

	// when
	_, err := config.Load(path)

	// then
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func Test_Load_shouldReturnErrParse_whenFileIsNotAConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		yml  string
	}{
		{name: "unknown top-level field", yml: validYAML + "aeras: []\n"},
		{name: "unknown area field", yml: "version: 1\nareas: [{name: auth, path: [svc-auth]}]\n"},
		{name: "empty file", yml: ""},
		{name: "malformed yaml", yml: "version: [1\n"},
		{name: "second document", yml: validYAML + "---\nversion: 1\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			path := writeRepo(t, tt.yml, "svc-auth", "svc-web", "shared/ui")

			// when
			_, err := config.Load(path)

			// then
			require.ErrorIs(t, err, config.ErrParse)
		})
	}
}

func Test_Load_shouldSucceed_whenFileEndsWithEmptyDocument(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, validYAML+"---\n", "svc-auth", "svc-web", "shared/ui")

	// when
	_, err := config.Load(path)

	// then
	require.NoError(t, err)
}

func Test_Load_shouldReturnErrInvalidConfig_whenParseFails(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, "version: [1\n")

	// when
	_, err := config.Load(path)

	// then
	require.ErrorIs(t, err, config.ErrInvalidConfig)
}

func Test_Load_shouldReturnErrInvalidConfig_whenFieldIsInvalid(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, "version: 2\nareas: [{name: a, paths: [a]}]\n", "a")

	// when
	_, err := config.Load(path)

	// then
	require.ErrorIs(t, err, config.ErrInvalidConfig)
}

func Test_Load_shouldNameYAMLKey_whenFieldIsInvalid(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, "version: 1\nareas: [{name: a, paths: []}]\n", "a")

	// when
	_, err := config.Load(path)

	// then
	require.ErrorContains(t, err, "Config.areas[0].paths")
}

func Test_Load_shouldReturnErrInvalidField_whenFieldIsInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		yml  string
	}{
		{name: "version missing", yml: "areas: [{name: a, paths: [a]}]\n"},
		{name: "version unsupported", yml: "version: 2\nareas: [{name: a, paths: [a]}]\n"},
		{name: "areas missing", yml: "version: 1\n"},
		{name: "area name missing", yml: "version: 1\nareas: [{paths: [a]}]\n"},
		{name: "area name duplicated", yml: "version: 1\nareas: [{name: a, paths: [a]}, {name: a, paths: [b]}]\n"},
		{name: "area paths missing", yml: "version: 1\nareas: [{name: a}]\n"},
		{name: "area path empty", yml: "version: 1\nareas: [{name: a, paths: [\"\"]}]\n"},
		{name: "exclude pattern empty", yml: "version: 1\nareas: [{name: a, paths: [a]}]\nexclude: [\"\"]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			path := writeRepo(t, tt.yml, "a", "b")

			// when
			_, err := config.Load(path)

			// then
			require.ErrorIs(t, err, config.ErrInvalidField)
		})
	}
}

func Test_Load_shouldReturnErrInvalidAreaPath_whenPathLeavesRepository(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{name: "parent", path: "../a"},
		{name: "absolute", path: "/a"},
		{name: "parent after clean", path: "a/../../b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			path := writeRepo(t, "version: 1\nareas: [{name: a, paths: [\""+tt.path+"\"]}]\n", "a")

			// when
			_, err := config.Load(path)

			// then
			require.ErrorIs(t, err, config.ErrInvalidAreaPath)
		})
	}
}

func Test_Load_shouldReturnErrAreaPathNotFound_whenDirectoryIsMissing(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, "version: 1\nareas: [{name: a, paths: [missing]}]\n")

	// when
	_, err := config.Load(path)

	// then
	require.ErrorIs(t, err, config.ErrAreaPathNotFound)
}

func Test_Load_shouldReturnErrAreaPathNotFound_whenPathIsFile(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, "version: 1\nareas: [{name: a, paths: [repohealth.yml]}]\n")

	// when
	_, err := config.Load(path)

	// then
	require.ErrorIs(t, err, config.ErrAreaPathNotFound)
}

func Test_Load_shouldReturnErrInvalidAreaPath_whenSymlinkLeavesRepository(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, "version: 1\nareas: [{name: a, paths: [link]}]\n")
	require.NoError(t, os.Symlink(t.TempDir(), filepath.Join(filepath.Dir(path), "link")))

	// when
	_, err := config.Load(path)

	// then
	require.ErrorIs(t, err, config.ErrInvalidAreaPath)
}

func Test_Load_shouldReturnErrInvalidAreaPath_whenSymlinkStaysInRepository(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
	}{
		{name: "link itself", path: "link"},
		{name: "directory under link", path: "link/b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			path := writeRepo(t, "version: 1\nareas: [{name: a, paths: [\""+tt.path+"\"]}]\n", "a/b")
			require.NoError(t, os.Symlink("a", filepath.Join(filepath.Dir(path), "link")))

			// when
			_, err := config.Load(path)

			// then
			require.ErrorIs(t, err, config.ErrInvalidAreaPath)
		})
	}
}

func Test_Load_shouldReturnErrAreaPathNotFound_whenParentIsFile(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, "version: 1\nareas: [{name: a, paths: [repohealth.yml/sub]}]\n")

	// when
	_, err := config.Load(path)

	// then
	require.ErrorIs(t, err, config.ErrAreaPathNotFound)
}

func Test_Load_shouldReportEveryProblem_whenSeveralAreInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want error
	}{
		{name: "missing directory", want: config.ErrAreaPathNotFound},
		{name: "path outside repository", want: config.ErrInvalidAreaPath},
		{name: "malformed exclude", want: config.ErrInvalidPattern},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			path := writeRepo(t, "version: 1\nareas: [{name: a, paths: [missing, ../b]}]\nexclude: [\"a/[b\"]\n")

			// when
			_, err := config.Load(path)

			// then
			require.ErrorIs(t, err, tt.want)
		})
	}
}

func Test_Load_shouldReturnErrAreaOverlap_whenPathsNest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		yml  string
	}{
		{name: "child after parent", yml: "version: 1\nareas: [{name: a, paths: [a]}, {name: b, paths: [a/b]}]\n"},
		{name: "parent after child", yml: "version: 1\nareas: [{name: a, paths: [a/b]}, {name: b, paths: [a]}]\n"},
		{name: "same path", yml: "version: 1\nareas: [{name: a, paths: [a]}, {name: b, paths: [./a/]}]\n"},
		{name: "repository root", yml: "version: 1\nareas: [{name: a, paths: [.]}, {name: b, paths: [a]}]\n"},
		{name: "same area", yml: "version: 1\nareas: [{name: a, paths: [a, a/b]}]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			path := writeRepo(t, tt.yml, "a/b")

			// when
			_, err := config.Load(path)

			// then
			require.ErrorIs(t, err, config.ErrAreaOverlap)
		})
	}
}

func Test_Load_shouldSucceed_whenPathsShareNamePrefix(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, "version: 1\nareas: [{name: a, paths: [svc]}, {name: b, paths: [svc-web]}]\n", "svc", "svc-web")

	// when
	_, err := config.Load(path)

	// then
	require.NoError(t, err)
}

func Test_Load_shouldReturnErrInvalidPattern_whenExcludeIsMalformed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
	}{
		{name: "unclosed bracket", pattern: "a/[b"},
		{name: "absolute", pattern: "/gen/**"},
		{name: "dot slash", pattern: "./gen/**"},
		{name: "parent", pattern: "../gen/**"},
		{name: "empty element", pattern: "gen//x.go"},
		{name: "trailing slash", pattern: "gen/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			path := writeRepo(t, "version: 1\nareas: [{name: a, paths: [a]}]\nexclude: [\""+tt.pattern+"\"]\n", "a")

			// when
			_, err := config.Load(path)

			// then
			require.ErrorIs(t, err, config.ErrInvalidPattern)
		})
	}
}

func Test_Load_shouldReturnErrInvalidConfig_whenAreaPathIsInvalid(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, "version: 1\nareas: [{name: a, paths: [missing]}]\n")

	// when
	_, err := config.Load(path)

	// then
	require.ErrorIs(t, err, config.ErrInvalidConfig)
}
