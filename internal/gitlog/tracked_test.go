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

func Test_TrackedFiles_shouldReturnIndexedFiles_whenDirIsRepositoryRoot(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory)

	// when
	files, err := gitlog.TrackedFiles(t.Context(), dir)

	// then
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"README.md", "docs/main.md", "svc-a/a.go", "svc-b/b.go"}, files)
}

func Test_TrackedFiles_shouldReturnPathsRelativeToDir_whenDirIsSubdirectory(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory)
	sub := filepath.Join(dir, "svc-a")
	require.NoError(t, os.Mkdir(sub, 0o750))

	// when
	files, err := gitlog.TrackedFiles(t.Context(), sub)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"a.go"}, files)
}

func Test_TrackedFiles_shouldKeepFileName_whenPathContainsSpecialCharacters(t *testing.T) {
	t.Parallel()

	// given
	dir := newRepo(t, baseHistory+separatorHistory)

	// when
	files, err := gitlog.TrackedFiles(t.Context(), dir)

	// then
	require.NoError(t, err)
	assert.Contains(t, files, "svc-a/x\x1eDEADBEEF\x1f2026-01-01T00:00:00Z")
}

func Test_TrackedFiles_shouldReturnExitError_whenDirIsNotRepository(t *testing.T) {
	t.Parallel()

	// given
	dir := t.TempDir()

	// when
	_, err := gitlog.TrackedFiles(t.Context(), dir)

	// then
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
}
