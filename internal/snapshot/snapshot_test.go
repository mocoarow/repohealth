package snapshot_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/complexity"
	"github.com/mocoarow/repohealth/internal/config"
	"github.com/mocoarow/repohealth/internal/gitlog"
	"github.com/mocoarow/repohealth/internal/hotspot"
	"github.com/mocoarow/repohealth/internal/snapshot"
	"github.com/mocoarow/repohealth/internal/source"
)

const (
	outputDir  = ".repohealth"
	commitDate = "2026-11-01T09:00:00+09:00"
	goMod      = "module example.com/r\n\ngo 1.26\n"
	// Two branches give a cognitive complexity of 2, which hotspots count.
	branchySource = "package app\n\nfunc F(a, b bool) int {\n\tif a {\n\t\treturn 1\n\t}\n\n\tif b {\n\t\treturn 2\n\t}\n\n\treturn 0\n}\n"
	goSource      = "package app\n\nfunc F(n int) int {\n\tif n > 0 {\n\t\treturn n\n\t}\n\n\treturn 0\n}\n"
)

func takenAt() time.Time {
	return time.Date(2026, 11, 23, 10, 0, 0, 0, time.UTC)
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()

	gitAt(t, dir, commitDate, args...)
}

func gitAt(t *testing.T, dir, date string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", args...) //nolint:gosec // test-controlled argv
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_AUTHOR_DATE="+date,
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com", "GIT_COMMITTER_DATE="+date)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
}

func commit(t *testing.T, dir string, files map[string]string) {
	t.Helper()

	commitAt(t, dir, commitDate, files)
}

func commitAt(t *testing.T, dir, date string, files map[string]string) {
	t.Helper()

	writeFiles(t, dir, files)
	git(t, dir, "add", "-A")
	gitAt(t, dir, date, "commit", "-q", "-m", "change")
}

func newRepo(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	commit(t, dir, files)

	return dir
}

func goRepo(t *testing.T) string {
	t.Helper()

	return newRepo(t, map[string]string{"go.mod": goMod, "app/a.go": goSource})
}

func appConfig(root string) config.Config {
	return config.Config{Root: root, Version: 1, Areas: []config.Area{{Name: "app", Paths: []string{"app"}}}}
}

func goFunction(file string) complexity.Function {
	return complexity.Function{File: file, Line: 3, Name: "F", Cyclo: 2, Cognit: 1}
}

func input(cfg config.Config) snapshot.Input {
	return snapshot.Input{Config: cfg, TakenAt: takenAt(), Version: "v0.0.0-test", OutputDir: outputDir}
}

func Test_Take_shouldReturnSameResult_whenTakenTwice(t *testing.T) {
	t.Parallel()

	// given
	in := input(appConfig(goRepo(t)))
	firstSummary, firstDetail, err := snapshot.Take(t.Context(), in)
	require.NoError(t, err)

	// when
	secondSummary, secondDetail, err := snapshot.Take(t.Context(), in)

	// then
	require.NoError(t, err)
	assert.Equal(t, []any{firstSummary, firstDetail}, []any{secondSummary, secondDetail})
}

func gitOnlyPath(t *testing.T) string {
	t.Helper()

	gitPath, err := exec.LookPath("git")
	require.NoError(t, err)

	bin := t.TempDir()
	require.NoError(t, os.Symlink(gitPath, filepath.Join(bin, "git")))

	return bin
}

func Test_Take_shouldSucceed_whenBiomeIsMissingAndRepositoryHasNoScripts(t *testing.T) {
	// given
	in := input(appConfig(goRepo(t)))
	t.Setenv("PATH", gitOnlyPath(t))

	// when
	_, detail, err := snapshot.Take(t.Context(), in)

	// then
	require.NoError(t, err)
	assert.Empty(t, detail.Tools.Biome)
}

func Test_Take_shouldSucceed_whenBiomeIsMissingAndScriptsAreTestsOnly(t *testing.T) {
	// given
	in := input(appConfig(newRepo(t, map[string]string{"app/a.test.ts": "export const x = 1;\n"})))
	t.Setenv("PATH", gitOnlyPath(t))

	// when
	_, detail, err := snapshot.Take(t.Context(), in)

	// then
	require.NoError(t, err)
	assert.Empty(t, detail.Tools.Biome)
}

func Test_Take_shouldCountRecentChurnFromHeadCommit_whenHeadIsOlderThanRecentDays(t *testing.T) {
	t.Parallel()

	// given
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	commitAt(t, dir, "2024-01-01T09:00:00+09:00", map[string]string{"go.mod": goMod, "app/a.go": branchySource})
	commitAt(t, dir, "2024-05-01T09:00:00+09:00", map[string]string{"app/a.go": branchySource + "\nfunc G() {}\n"})

	// when
	_, detail, err := snapshot.Take(t.Context(), input(appConfig(dir)))

	// then
	require.NoError(t, err)
	assert.Equal(t, []hotspot.File{{Path: "app/a.go", Churn: 2, RecentChurn: 1, SumCognit: 2, MaxCognit: 2, Score: 4}}, detail.Hotspots)
}

func Test_Take_shouldReturnErrDirtyWorktree_whenTrackedFileChanges(t *testing.T) {
	t.Parallel()

	// given
	dir := goRepo(t)
	writeFiles(t, dir, map[string]string{"app/a.go": goSource + "\nfunc G() {}\n"})

	// when
	_, _, err := snapshot.Take(t.Context(), input(appConfig(dir)))

	// then
	require.ErrorIs(t, err, gitlog.ErrDirtyWorktree)
}

func Test_Take_shouldSucceed_whenOnlyOutputDirectoryChanges(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, map[string]string{"go.mod": goMod, "app/a.go": goSource, outputDir + "/history.jsonl": "{}\n"})
	writeFiles(t, dir, map[string]string{outputDir + "/history.jsonl": "{}\n{}\n"})

	// when
	_, _, err := snapshot.Take(t.Context(), input(appConfig(dir)))

	// then
	require.NoError(t, err)
}

func Test_Take_shouldReturnErrShallowRepository_whenCloneIsShallow(t *testing.T) {
	t.Parallel()

	// given
	origin := goRepo(t)
	commit(t, origin, map[string]string{"app/b.go": "package app\n"})

	clone := filepath.Join(t.TempDir(), "clone")
	git(t, origin, "clone", "-q", "--depth", "1", "file://"+origin, clone)

	// when
	_, _, err := snapshot.Take(t.Context(), input(appConfig(clone)))

	// then
	require.ErrorIs(t, err, gitlog.ErrShallowRepository)
}

func Test_Take_shouldMeasureHeadOnly_whenConfigIsInSubdirectory(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, map[string]string{
		"app/outside.go": goSource,
		"sub/go.mod":     goMod,
		"sub/app/a.go":   goSource,
	})
	writeFiles(t, dir, map[string]string{"sub/app/untracked.go": goSource})

	// when
	_, detail, err := snapshot.Take(t.Context(), input(appConfig(filepath.Join(dir, "sub"))))

	// then
	require.NoError(t, err)
	assert.Equal(t, []snapshot.Function{{Lang: source.Go, Function: goFunction("app/a.go")}}, detail.Functions)
}

func Test_Take_shouldRecordBiomeVersion_whenAreaHasScripts(t *testing.T) {
	t.Parallel()

	// given
	root := newRepo(t, map[string]string{"pkg/web/a.ts": "export const x = 1;\n"})
	cfg := config.Config{Root: root, Version: 1, Areas: []config.Area{{Name: "frontend", Paths: []string{"pkg/web"}}}}
	want, err := complexity.BiomeVersion(t.Context(), filepath.Join(root, "pkg", "web"))
	require.NoError(t, err)

	// when
	_, detail, err := snapshot.Take(t.Context(), input(cfg))

	// then
	require.NoError(t, err)
	assert.Equal(t, want, detail.Tools.Biome)
}
