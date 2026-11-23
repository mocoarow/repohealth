package gitlog_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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

func Test_Churn_shouldCountNonMergeCommitsPerFile_whenFilesChangeRepeatedly(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory+secondChangeHistory)

	// when
	churn, err := gitlog.Churn(t.Context(), dir)

	// then
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"README.md": 1, "docs/main.md": 1, "svc-a/a.go": 2, "svc-b/b.go": 1}, churn)
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
	churn, err := gitlog.Churn(t.Context(), dir)

	// then
	require.NoError(t, err)
	assert.Equal(t, 1, churn["README.md"])
}

func Test_Churn_shouldReturnPathsRelativeToDir_whenDirIsSubdirectory(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory+secondChangeHistory)
	sub := filepath.Join(dir, "svc-a")
	require.NoError(t, os.Mkdir(sub, 0o750))

	// when
	churn, err := gitlog.Churn(t.Context(), sub)

	// then
	require.NoError(t, err)
	assert.Equal(t, map[string]int{"a.go": 2}, churn)
}

func Test_Churn_shouldKeepFileName_whenPathContainsSpecialCharacters(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory+separatorHistory)

	// when
	churn, err := gitlog.Churn(t.Context(), dir)

	// then
	require.NoError(t, err)
	assert.Equal(t, 1, churn["svc-a/x\x1eDEADBEEF\x1f2026-01-01T00:00:00Z"])
}

func Test_Churn_shouldReturnExitError_whenDirIsNotRepository(t *testing.T) {
	t.Parallel()

	// given
	dir := t.TempDir()

	// when
	_, err := gitlog.Churn(t.Context(), dir)

	// then
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
}
