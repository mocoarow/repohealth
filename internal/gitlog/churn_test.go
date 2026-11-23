package gitlog_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/gitlog"
)

const secondChangeHistory = `commit refs/heads/main
mark :7
committer t <t@example.com> DATE
data 0
from :6
M 100644 inline svc-a/a.go
data 2
a2
`

const outsideChangeHistory = `commit refs/heads/main
mark :8
committer t <t@example.com> DATE
data 0
from :7
M 100644 inline README.md
data 2
r2
`

// outOfOrderHistory: the middle commit is older than both its parent and its child.
const outOfOrderHistory = `commit refs/heads/main
mark :1
committer t <t@example.com> DATE
data 0
M 100644 inline README.md
data 1
r
commit refs/heads/main
mark :2
committer t <t@example.com> OLDDATE
data 0
from :1
M 100644 inline svc-a/a.go
data 1
a
commit refs/heads/main
mark :3
committer t <t@example.com> DATE
data 0
from :2
M 100644 inline svc-a/a.go
data 2
a2
`

var recentSince = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func totals(churn map[string]gitlog.FileChurn) map[string]int {
	result := map[string]int{}
	for name, c := range churn {
		result[name] = c.Total
	}

	return result
}

func Test_Churn_shouldCountNonMergeCommitsPerFile_whenFilesChangeRepeatedly(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory+secondChangeHistory)

	// when
	churn, err := gitlog.Churn(t.Context(), dir, headRev(t, dir), recentSince)

	// then
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"README.md": 1, "docs/main.md": 1, "svc-a/a.go": 2, "svc-b/b.go": 1}, totals(churn))
}

func Test_Churn_shouldCountRootCommit_whenShowRootIsDisabled(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory)
	configCmd := exec.CommandContext(t.Context(), "git", "config", "log.showRoot", "false")
	configCmd.Dir = dir
	out, err := configCmd.CombinedOutput()
	require.NoError(t, err, string(out))

	// when
	churn, err := gitlog.Churn(t.Context(), dir, headRev(t, dir), recentSince)

	// then
	require.NoError(t, err)
	assert.Equal(t, 1, churn["README.md"].Total)
}

func Test_Churn_shouldReturnPathsRelativeToDir_whenDirIsSubdirectory(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory+secondChangeHistory)
	sub := filepath.Join(dir, "svc-a")
	require.NoError(t, os.Mkdir(sub, 0o750))

	// when
	churn, err := gitlog.Churn(t.Context(), sub, headRev(t, sub), recentSince)

	// then
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"a.go": 2}, totals(churn))
}

func Test_Churn_shouldSkipCommit_whenItChangesNothingUnderDir(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory+secondChangeHistory+outsideChangeHistory)
	sub := filepath.Join(dir, "svc-a")
	require.NoError(t, os.Mkdir(sub, 0o750))

	// when
	churn, err := gitlog.Churn(t.Context(), sub, headRev(t, sub), recentSince)

	// then
	require.NoError(t, err)
	assert.Equal(t, map[string]gitlog.FileChurn{"a.go": {Total: 2, Recent: 2}}, churn)
}

func Test_Churn_shouldKeepFileName_whenPathContainsSpecialCharacters(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory+separatorHistory)

	// when
	churn, err := gitlog.Churn(t.Context(), dir, headRev(t, dir), recentSince)

	// then
	require.NoError(t, err)
	assert.Equal(t, 1, churn["svc-a/x\x1eDEADBEEF\x1f2026-01-01T00:00:00Z"].Total)
}

func Test_Churn_shouldCountRecentCommits_whenCommitDatesAreOutOfOrder(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, outOfOrderHistory)

	// when
	churn, err := gitlog.Churn(t.Context(), dir, headRev(t, dir), recentSince)

	// then
	require.NoError(t, err)
	assert.Equal(t, map[string]gitlog.FileChurn{"README.md": {Total: 1, Recent: 1}, "svc-a/a.go": {Total: 2, Recent: 1}}, churn)
}

func Test_Churn_shouldCountCommitAsRecent_whenCommittedAtEqualsSince(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, noMergeHistory)
	since, err := time.Parse(time.RFC3339, mergeDate)
	require.NoError(t, err)

	// when
	churn, err := gitlog.Churn(t.Context(), dir, headRev(t, dir), since)

	// then
	require.NoError(t, err)
	assert.Equal(t, 1, churn["README.md"].Recent)
}

func Test_ParseChurnHeader_shouldReturnErrUnexpectedOutput_whenHeaderIsMalformed(t *testing.T) {
	t.Parallel()

	const hash = "0123456789abcdef0123456789abcdef01234567"

	tests := []struct {
		name  string
		token string
	}{
		{name: "no date", token: "\x1e" + hash},
		{name: "short hash", token: "\x1eabc\x1f2026-10-31T23:30:00+09:00"},
		{name: "malformed date", token: "\x1e" + hash + "\x1f2026-10-31"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			token := tt.token

			// when
			err := gitlog.ParseChurnHeader(token)

			// then
			require.ErrorIs(t, err, gitlog.ErrUnexpectedOutput)
		})
	}
}

func Test_Churn_shouldReturnExitError_whenDirIsNotRepository(t *testing.T) {
	t.Parallel()

	// given
	dir := t.TempDir()

	// when
	_, err := gitlog.Churn(t.Context(), dir, "HEAD", recentSince)

	// then
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
}
