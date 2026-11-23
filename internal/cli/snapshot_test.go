package cli_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/cli"
	"github.com/mocoarow/repohealth/internal/config"
	"github.com/mocoarow/repohealth/internal/gitlog"
)

const (
	goMod      = "module example.com/m\n\ngo 1.22\n"
	goSource   = "package a\n\nfunc A() int { return 1 }\n"
	configYAML = "version: 1\nareas: [{name: a, paths: [a]}]\n"
	historyRel = ".repohealth/history.jsonl"
)

var takenAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", args...) //nolint:gosec // test-controlled argv
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))

	return strings.TrimSpace(string(out))
}

func writeRepoFile(t *testing.T, root, name, content string) {
	t.Helper()

	path := filepath.Join(root, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func commitAll(t *testing.T, root string) {
	t.Helper()

	runGit(t, root, "add", "-A")
	runGit(t, root, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "c")
}

// newGoRepo returns the root of a clean git repository holding one Go area; extra files are committed with it.
func newGoRepo(t *testing.T, extra map[string]string) string {
	t.Helper()

	root := t.TempDir()
	runGit(t, root, "init", "-q", "-b", "main")
	writeRepoFile(t, root, "go.mod", goMod)
	writeRepoFile(t, root, "a/a.go", goSource)
	writeRepoFile(t, root, config.FileName, configYAML)

	for name, content := range extra {
		writeRepoFile(t, root, name, content)
	}

	commitAll(t, root)

	return root
}

func snapshotArgs(root string) []string {
	return []string{"snapshot", "-config", filepath.Join(root, config.FileName)}
}

func countLines(t *testing.T, path string) int {
	t.Helper()

	data, err := os.ReadFile(path) //nolint:gosec // test-controlled path
	require.NoError(t, err)

	return strings.Count(string(data), "\n")
}

func Test_Run_shouldSaveSnapshot_whenRepositoryIsClean(t *testing.T) {
	t.Parallel()

	// given
	root := newGoRepo(t, nil)

	// when
	err := cli.RunAt(t.Context(), takenAt, "v1", snapshotArgs(root), &bytes.Buffer{}, &bytes.Buffer{})

	// then
	require.NoError(t, err)
	assert.Equal(t, 1, countLines(t, filepath.Join(root, historyRel)))
}

func Test_Run_shouldPrintRevAndDirectory_whenSnapshotIsSaved(t *testing.T) {
	t.Parallel()

	// given
	root := newGoRepo(t, nil)
	head, err := gitlog.Head(t.Context(), root)
	require.NoError(t, err)

	var stdout bytes.Buffer

	// when
	err = cli.RunAt(t.Context(), takenAt, "v1", snapshotArgs(root), &stdout, &bytes.Buffer{})

	// then
	require.NoError(t, err)
	assert.Equal(t, head.Hash+": saved to "+filepath.Join(root, ".repohealth")+"\n", stdout.String())
}

func Test_Run_shouldRecordVersion_whenSnapshotIsSaved(t *testing.T) {
	t.Parallel()

	// given
	root := newGoRepo(t, nil)

	// when
	err := cli.RunAt(t.Context(), takenAt, "v1.2.3", snapshotArgs(root), &bytes.Buffer{}, &bytes.Buffer{})

	// then
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(root, historyRel)) //nolint:gosec // test-controlled path
	require.NoError(t, err)
	assert.Contains(t, string(data), `"repohealth":"v1.2.3"`)
}

func Test_Run_shouldRecordTakenAt_whenSnapshotIsSaved(t *testing.T) {
	t.Parallel()

	// given
	root := newGoRepo(t, nil)

	// when
	err := cli.RunAt(t.Context(), takenAt, "v1.2.3", snapshotArgs(root), &bytes.Buffer{}, &bytes.Buffer{})

	// then
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(root, historyRel)) //nolint:gosec // test-controlled path
	require.NoError(t, err)
	assert.Contains(t, string(data), `"takenAt":"2026-01-02T03:04:05Z"`)
}

func Test_Run_shouldAppendSnapshot_whenOnlyHistoryChanged(t *testing.T) {
	t.Parallel()

	// given
	root := newGoRepo(t, nil)
	require.NoError(t, cli.RunAt(t.Context(), takenAt, "v1", snapshotArgs(root), &bytes.Buffer{}, &bytes.Buffer{}))
	commitAll(t, root)

	// when
	err := cli.RunAt(t.Context(), takenAt, "v1", snapshotArgs(root), &bytes.Buffer{}, &bytes.Buffer{})

	// then
	require.NoError(t, err)
	assert.Equal(t, 2, countLines(t, filepath.Join(root, historyRel)))
}

func Test_Run_shouldReturnErrDirtyWorktree_whenTrackedFileChanges(t *testing.T) {
	t.Parallel()

	// given
	root := newGoRepo(t, nil)
	writeRepoFile(t, root, "a/a.go", goSource+"\n// changed\n")

	// when
	err := cli.RunAt(t.Context(), takenAt, "v1", snapshotArgs(root), &bytes.Buffer{}, &bytes.Buffer{})

	// then
	require.ErrorIs(t, err, gitlog.ErrDirtyWorktree)
}

func Test_Run_shouldWarn_whenCoverageRecordsAreSkipped(t *testing.T) {
	t.Parallel()

	// given
	lcov := "SF:../outside.go\nDA:1,1\nend_of_record\nSF:a/a.go\nDA:3,1\nend_of_record\n"
	root := newGoRepo(t, map[string]string{
		"coverage.lcov": lcov,
		config.FileName: configYAML + "coverage:\n  - file: coverage.lcov\n",
	})

	var stderr bytes.Buffer

	// when
	err := cli.RunAt(t.Context(), takenAt, "v1", snapshotArgs(root), &bytes.Buffer{}, &stderr)

	// then
	require.NoError(t, err)
	assert.Equal(t, "warning: coverage.lcov: 1 source records outside the config directory skipped\n", stderr.String())
}

func Test_Run_shouldReturnErrUsage_whenSnapshotArgumentsAreInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "unknown flag", args: []string{"snapshot", "-nope"}},
		{name: "positional argument", args: []string{"snapshot", "extra"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			var stderr bytes.Buffer

			// when
			err := cli.Run(t.Context(), tt.args, &bytes.Buffer{}, &stderr)

			// then
			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Contains(t, stderr.String(), "usage: repohealth snapshot")
		})
	}
}

func Test_Run_shouldPrintFlags_whenSnapshotHelpIsRequested(t *testing.T) {
	t.Parallel()

	// given
	var stdout bytes.Buffer

	// when
	err := cli.Run(t.Context(), []string{"snapshot", "-h"}, &stdout, &bytes.Buffer{})

	// then
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "-config")
}
