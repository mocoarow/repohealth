package gitlog_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/gitlog"
)

const mergeDate = "2026-10-31T23:30:00+09:00"

// baseHistory: main merges feature at :6; feature's own merge of main (:4) is off main's first parent.
const baseHistory = `commit refs/heads/main
mark :1
committer t <t@example.com> DATE
data 0
M 100644 inline README.md
data 1
r
commit refs/heads/feature
mark :2
committer t <t@example.com> DATE
data 0
from :1
M 100644 inline svc-a/a.go
data 1
a
commit refs/heads/main
mark :3
committer t <t@example.com> DATE
data 0
from :1
M 100644 inline docs/main.md
data 1
d
commit refs/heads/feature
mark :4
committer t <t@example.com> DATE
data 0
from :2
merge :3
M 100644 inline docs/main.md
data 1
d
commit refs/heads/feature
mark :5
committer t <t@example.com> DATE
data 0
from :4
M 100644 inline svc-b/b.go
data 1
b
commit refs/heads/main
mark :6
committer t <t@example.com> DATE
data 0
from :3
merge :5
M 100644 inline svc-a/a.go
data 1
a
M 100644 inline svc-b/b.go
data 1
b
`

const moveHistory = `commit refs/heads/move
mark :7
committer t <t@example.com> DATE
data 0
from :6
D svc-a/a.go
M 100644 inline svc-b/a.go
data 1
a
commit refs/heads/main
mark :8
committer t <t@example.com> DATE
data 0
from :6
merge :7
D svc-a/a.go
M 100644 inline svc-b/a.go
data 1
a
`

const emptyHistory = `commit refs/heads/empty
mark :7
committer t <t@example.com> DATE
data 0
from :6
commit refs/heads/main
mark :8
committer t <t@example.com> DATE
data 0
from :6
merge :7
`

const separatorHistory = `commit refs/heads/odd
mark :7
committer t <t@example.com> DATE
data 0
from :6
M 100644 inline "svc-a/x\036DEADBEEF\0372026-01-01T00:00:00Z"
data 1
x
commit refs/heads/main
mark :8
committer t <t@example.com> DATE
data 0
from :6
merge :7
M 100644 inline "svc-a/x\036DEADBEEF\0372026-01-01T00:00:00Z"
data 1
x
`

const noMergeHistory = `commit refs/heads/main
mark :1
committer t <t@example.com> DATE
data 0
M 100644 inline README.md
data 1
r
`

func newRepo(t *testing.T, history string) string {
	t.Helper()

	date, err := time.Parse(time.RFC3339, mergeDate)
	require.NoError(t, err)

	dir := t.TempDir()

	initCmd := exec.CommandContext(t.Context(), "git", "init", "-q", "-b", "main")
	initCmd.Dir = dir
	out, err := initCmd.CombinedOutput()
	require.NoError(t, err, string(out))

	importCmd := exec.CommandContext(t.Context(), "git", "fast-import", "--quiet")
	importCmd.Dir = dir
	importCmd.Stdin = strings.NewReader(strings.ReplaceAll(history, "DATE", strconv.FormatInt(date.Unix(), 10)+" "+date.Format("-0700")))
	out, err = importCmd.CombinedOutput()
	require.NoError(t, err, string(out))

	return dir
}

func Test_Merges_shouldReturnFirstParentMergesOnly_whenBranchMergedMain(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory)

	// when
	merges, err := gitlog.Merges(t.Context(), dir)

	// then
	require.NoError(t, err)
	assert.Len(t, merges, 1)
}

func Test_Merges_shouldReturnFilesChangedAgainstFirstParent_whenMergeIsListed(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory)

	// when
	merges, err := gitlog.Merges(t.Context(), dir)

	// then
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"svc-a/a.go", "svc-b/b.go"}, merges[0].Files)
}

func Test_Merges_shouldKeepRecordedTimeZone_whenMergeIsListed(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory)

	// when
	merges, err := gitlog.Merges(t.Context(), dir)

	// then
	require.NoError(t, err)
	assert.Equal(t, mergeDate, merges[0].CommittedAt.Format(time.RFC3339))
}

func Test_Merges_shouldReturnPathsRelativeToDir_whenDirIsSubdirectory(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory)
	sub := filepath.Join(dir, "svc-a")
	require.NoError(t, os.Mkdir(sub, 0o750))

	// when
	merges, err := gitlog.Merges(t.Context(), sub)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"a.go"}, merges[0].Files)
}

func Test_Merges_shouldReturnBothPaths_whenFileMovesBetweenDirectories(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory+moveHistory)

	// when
	merges, err := gitlog.Merges(t.Context(), dir)

	// then
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"svc-a/a.go", "svc-b/a.go"}, merges[0].Files)
}

func Test_Merges_shouldReturnNoFiles_whenMergeChangesNothing(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory+emptyHistory)

	// when
	merges, err := gitlog.Merges(t.Context(), dir)

	// then
	require.NoError(t, err)
	assert.Empty(t, merges[0].Files)
}

func Test_Merges_shouldReturnNoMerges_whenHistoryHasNone(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, noMergeHistory)

	// when
	merges, err := gitlog.Merges(t.Context(), dir)

	// then
	require.NoError(t, err)
	assert.Empty(t, merges)
}

func Test_Merges_shouldKeepFileName_whenPathContainsRecordSeparators(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory+separatorHistory)

	// when
	merges, err := gitlog.Merges(t.Context(), dir)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"svc-a/x\x1eDEADBEEF\x1f2026-01-01T00:00:00Z"}, merges[0].Files)
}

func Test_ParseHeader_shouldReturnErrUnexpectedOutput_whenLineIsMalformed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
	}{
		{name: "text instead of header", line: "gpg: Signature made"},
		{name: "no parent", line: "abc\x1f\x1f2026-10-31T23:30:00+09:00"},
		{name: "malformed date", line: "abc\x1fdef\x1f2026-10-31"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			line := tt.line

			// when
			err := gitlog.ParseHeader(line)

			// then
			require.ErrorIs(t, err, gitlog.ErrUnexpectedOutput)
		})
	}
}

func Test_Merges_shouldReturnExitError_whenDirIsNotRepository(t *testing.T) {
	t.Parallel()

	// given
	dir := t.TempDir()

	// when
	_, err := gitlog.Merges(t.Context(), dir)

	// then
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
}
