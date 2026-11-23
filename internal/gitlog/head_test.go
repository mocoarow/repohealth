package gitlog_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/gitlog"
)

const ignoreDir = ".repohealth"

const nonRegularHistory = `commit refs/heads/main
mark :1
committer t <t@example.com> DATE
data 0
M 100644 inline README.md
data 1
r
M 100755 inline bin/run.sh
data 1
x
M 120000 inline link
data 9
README.md
M 160000 0123456789abcdef0123456789abcdef01234567 vendor/sub
`

const ignoredDirHistory = `commit refs/heads/main
mark :7
committer t <t@example.com> DATE
data 0
from :6
M 100644 inline .repohealth/history.jsonl
data 2
{}
`

// conflictHistory: main and other both rewrite x.txt, so merging other into main conflicts.
const conflictHistory = `commit refs/heads/main
mark :1
committer t <t@example.com> DATE
data 0
M 100644 inline x.txt
data 1
a
commit refs/heads/other
mark :2
committer t <t@example.com> DATE
data 0
from :1
M 100644 inline x.txt
data 1
c
commit refs/heads/main
mark :3
committer t <t@example.com> DATE
data 0
from :1
M 100644 inline x.txt
data 1
b
`

func git(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", args...) //nolint:gosec // test-controlled argv
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()

	return string(out), err
}

func mustGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	out, err := git(t, dir, args...)
	require.NoError(t, err, out)

	return out
}

func newCheckedOutRepo(t *testing.T, history string) string {
	t.Helper()

	dir := newRepo(t, history)
	mustGit(t, dir, "checkout-index", "-a", "-f")

	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func Test_Head_shouldReturnHashAndCommitterDate_whenRepositoryHasCommits(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory)
	want := strings.TrimSpace(mustGit(t, dir, "rev-parse", "HEAD"))

	// when
	head, err := gitlog.Head(t.Context(), dir)

	// then
	require.NoError(t, err)
	assert.Equal(t, gitlog.Commit{Hash: want, CommittedAt: mustParseTime(t, mergeDate)}, head)
}

func Test_Head_shouldReturnExitError_whenDirIsNotRepository(t *testing.T) {
	t.Parallel()

	// given
	dir := t.TempDir()

	// when
	_, err := gitlog.Head(t.Context(), dir)

	// then
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
}

func mustParseTime(t *testing.T, value string) time.Time {
	t.Helper()

	parsed, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err)

	return parsed
}

func Test_HeadFiles_shouldReturnRegularFiles_whenTreeHasSymlinkAndSubmodule(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, nonRegularHistory)

	// when
	files, err := gitlog.HeadFiles(t.Context(), dir, headRev(t, dir))

	// then
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"README.md", "bin/run.sh"}, files)
}

func Test_HeadFiles_shouldReturnPathsRelativeToDir_whenDirIsSubdirectory(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory)
	sub := filepath.Join(dir, "svc-a")
	require.NoError(t, os.Mkdir(sub, 0o750))

	// when
	files, err := gitlog.HeadFiles(t.Context(), sub, headRev(t, sub))

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"a.go"}, files)
}

func Test_HeadFiles_shouldReturnCommittedFiles_whenIndexDiffers(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory)
	mustGit(t, dir, "rm", "--cached", "-q", "README.md")

	// when
	files, err := gitlog.HeadFiles(t.Context(), dir, headRev(t, dir))

	// then
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"README.md", "docs/main.md", "svc-a/a.go", "svc-b/b.go"}, files)
}

func Test_HeadFiles_shouldKeepFileName_whenPathContainsSpecialCharacters(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory+separatorHistory)

	// when
	files, err := gitlog.HeadFiles(t.Context(), dir, headRev(t, dir))

	// then
	require.NoError(t, err)
	assert.Contains(t, files, "svc-a/x\x1eDEADBEEF\x1f2026-01-01T00:00:00Z")
}

func Test_HeadFiles_shouldReturnExitError_whenDirIsNotRepository(t *testing.T) {
	t.Parallel()

	// given
	dir := t.TempDir()

	// when
	_, err := gitlog.HeadFiles(t.Context(), dir, "HEAD")

	// then
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
}

func Test_CheckClean_shouldSucceed_whenWorktreeIsUnchanged(t *testing.T) {
	t.Parallel()

	// given
	dir := newCheckedOutRepo(t, baseHistory)

	// when
	err := gitlog.CheckClean(t.Context(), dir, ignoreDir)

	// then
	require.NoError(t, err)
}

func Test_CheckClean_shouldSucceed_whenOnlyIgnoredDirectoryChanges(t *testing.T) {
	t.Parallel()

	// given
	dir := newCheckedOutRepo(t, baseHistory+ignoredDirHistory)
	writeFile(t, filepath.Join(dir, ignoreDir, "history.jsonl"), "{\"changed\":true}\n")

	// when
	err := gitlog.CheckClean(t.Context(), dir, ignoreDir)

	// then
	require.NoError(t, err)
}

func Test_CheckClean_shouldReturnErrDirtyWorktree_whenTrackedFileChanges(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		history string
		change  func(t *testing.T, dir string)
	}{
		{
			name:    "worktree change",
			history: baseHistory,
			change: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "README.md"), "changed\n")
			},
		},
		{
			name:    "staged change",
			history: baseHistory,
			change: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, filepath.Join(dir, "README.md"), "changed\n")
				mustGit(t, dir, "add", "README.md")
			},
		},
		{
			name:    "unmerged",
			history: conflictHistory,
			change: func(t *testing.T, dir string) {
				t.Helper()
				_, err := git(t, dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "merge", "other")
				var exitErr *exec.ExitError
				require.ErrorAs(t, err, &exitErr)
				require.NotEmpty(t, mustGit(t, dir, "ls-files", "--unmerged"))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			dir := newCheckedOutRepo(t, tt.history)
			tt.change(t, dir)

			// when
			err := gitlog.CheckClean(t.Context(), dir, ignoreDir)

			// then
			require.ErrorIs(t, err, gitlog.ErrDirtyWorktree)
		})
	}
}

func Test_CheckClean_shouldReturnErrDirtyWorktree_whenIgnoreIsEmpty(t *testing.T) {
	t.Parallel()

	// given
	dir := newCheckedOutRepo(t, baseHistory)
	writeFile(t, filepath.Join(dir, "README.md"), "changed\n")

	// when
	err := gitlog.CheckClean(t.Context(), dir, "")

	// then
	require.ErrorIs(t, err, gitlog.ErrDirtyWorktree)
}

func Test_CheckClean_shouldSucceed_whenChangeIsOutsideDir(t *testing.T) {
	t.Parallel()

	// given
	dir := newCheckedOutRepo(t, baseHistory)
	writeFile(t, filepath.Join(dir, "README.md"), "changed\n")

	// when
	err := gitlog.CheckClean(t.Context(), filepath.Join(dir, "svc-a"), ignoreDir)

	// then
	require.NoError(t, err)
}

func Test_CheckClean_shouldReturnExitError_whenDirIsNotRepository(t *testing.T) {
	t.Parallel()

	// given
	dir := t.TempDir()

	// when
	err := gitlog.CheckClean(t.Context(), dir, ignoreDir)

	// then
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
}

func Test_CheckComplete_shouldReturnErrShallowRepository_whenCloneIsShallow(t *testing.T) {
	t.Parallel()

	// given
	src := newRepo(t, baseHistory)
	clone := filepath.Join(t.TempDir(), "clone")
	mustGit(t, filepath.Dir(clone), "clone", "-q", "--depth", "1", "file://"+src, clone)

	// when
	err := gitlog.CheckComplete(t.Context(), clone)

	// then
	require.ErrorIs(t, err, gitlog.ErrShallowRepository)
}

func Test_CheckComplete_shouldSucceed_whenHistoryIsComplete(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory)

	// when
	err := gitlog.CheckComplete(t.Context(), dir)

	// then
	require.NoError(t, err)
}

func Test_CheckClean_shouldNameChangedFile_whenTrackedFileChanges(t *testing.T) {
	t.Parallel()

	// given
	dir := newCheckedOutRepo(t, baseHistory)
	writeFile(t, filepath.Join(dir, "README.md"), "changed\n")

	// when
	err := gitlog.CheckClean(t.Context(), dir, ignoreDir)

	// then
	require.EqualError(t, err, "worktree has uncommitted changes: README.md")
}

func Test_CheckClean_shouldNameOnlyNewPath_whenFileIsRenamed(t *testing.T) {
	t.Parallel()

	// given
	dir := newCheckedOutRepo(t, baseHistory)
	mustGit(t, dir, "mv", "README.md", "GUIDE.md")

	// when
	err := gitlog.CheckClean(t.Context(), dir, ignoreDir)

	// then
	require.EqualError(t, err, "worktree has uncommitted changes: GUIDE.md")
}

func Test_CheckClean_shouldNameFirstFivePaths_whenMoreFilesChanged(t *testing.T) {
	t.Parallel()

	// given
	dir := newCheckedOutRepo(t, baseHistory)

	for _, name := range []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt", "f.txt", "g.txt"} {
		writeFile(t, filepath.Join(dir, name), "new\n")
		mustGit(t, dir, "add", name)
	}

	// when
	err := gitlog.CheckClean(t.Context(), dir, ignoreDir)

	// then
	require.EqualError(t, err, "worktree has uncommitted changes: a.txt, b.txt, c.txt, d.txt, e.txt (+2 more)")
}
