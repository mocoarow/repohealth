package complexity_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/complexity"
)

func Test_BiomeVersion_shouldReturnVersion_whenBiomeIsInstalled(t *testing.T) {
	t.Parallel()

	// given
	dir := "."

	// when
	got, err := complexity.BiomeVersion(t.Context(), dir)

	// then
	require.NoError(t, err)
	assert.NotEmpty(t, got)
}

func Test_BiomeVersion_shouldReturnBareNumber_whenBiomeIsInstalled(t *testing.T) {
	t.Parallel()

	// given
	dir := "."

	// when
	got, err := complexity.BiomeVersion(t.Context(), dir)

	// then
	require.NoError(t, err)
	assert.NotContains(t, got, "Version:")
}

func Test_ParseBiomeVersion_shouldReturnErrUnexpectedBiomeOutput_whenOutputIsUnknown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		out  string
	}{
		{name: "empty", out: ""},
		{name: "no version label", out: "2.4.10"},
		{name: "empty version", out: "Version:  \n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			// when
			err := complexity.ParseBiomeVersion(tt.out)

			// then
			require.ErrorIs(t, err, complexity.ErrUnexpectedBiomeOutput)
		})
	}
}
