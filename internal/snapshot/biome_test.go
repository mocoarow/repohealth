package snapshot_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/snapshot"
)

func Test_OneVersion_shouldReturnVersion_whenEveryDirectoryAgrees(t *testing.T) {
	t.Parallel()

	// given
	versions := map[string]string{"/r/web": "2.4.10", "/r/admin": "2.4.10"}

	// when
	got, err := snapshot.OneVersion(versions)

	// then
	require.NoError(t, err)
	assert.Equal(t, "2.4.10", got)
}

func Test_OneVersion_shouldReturnEmpty_whenNoDirectoryHasVersion(t *testing.T) {
	t.Parallel()

	// given
	versions := map[string]string{}

	// when
	got, err := snapshot.OneVersion(versions)

	// then
	require.NoError(t, err)
	assert.Empty(t, got)
}

func Test_OneVersion_shouldReturnErrBiomeVersionMismatch_whenVersionsDiffer(t *testing.T) {
	t.Parallel()

	// given
	versions := map[string]string{"/r/web": "2.4.10", "/r/admin": "2.3.0"}

	// when
	_, err := snapshot.OneVersion(versions)

	// then
	require.ErrorIs(t, err, snapshot.ErrBiomeVersionMismatch)
}

func Test_OneVersion_shouldNameVersionsAndSortedDirectories_whenVersionsDiffer(t *testing.T) {
	t.Parallel()

	// given
	versions := map[string]string{"/r/web": "2.3.0", "/r/app": "2.4.10", "/r/admin": "2.4.10"}

	// when
	_, err := snapshot.OneVersion(versions)

	// then
	require.EqualError(t, err,
		"biome version differs between areas: 2.3.0 (/r/web); 2.4.10 (/r/admin, /r/app)")
}
