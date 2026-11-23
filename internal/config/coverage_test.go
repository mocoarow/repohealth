package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/config"
)

func Test_Load_shouldReturnCoverageReports_whenPathsAreLocal(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, twoAreas+"coverage: [{file: coverage.lcov}, {file: web/coverage/lcov.info, root: web}]\n", "auth", "web")

	// when
	cfg, err := config.Load(path)

	// then
	require.NoError(t, err)
	assert.Equal(t, []config.Report{{File: "coverage.lcov", Root: ""}, {File: "web/coverage/lcov.info", Root: "web"}}, cfg.Coverage)
}

func Test_Load_shouldReturnErrInvalidCoveragePath_whenPathIsNotCleanLocal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		report string
	}{
		{name: "absolute file", report: "{file: /tmp/c.lcov}"},
		{name: "parent file", report: "{file: ../c.lcov}"},
		{name: "parent root", report: "{file: c.lcov, root: ..}"},
		{name: "dot slash file", report: "{file: ./c.lcov}"},
		{name: "trailing slash root", report: "{file: c.lcov, root: web/}"},
		{name: "dot file", report: "{file: .}"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			path := writeRepo(t, twoAreas+"coverage: ["+tt.report+"]\n", "auth", "web")

			// when
			_, err := config.Load(path)

			// then
			require.ErrorIs(t, err, config.ErrInvalidCoveragePath)
		})
	}
}

func Test_Load_shouldReturnErrInvalidField_whenCoverageFileIsMissing(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, twoAreas+"coverage: [{root: web}]\n", "auth", "web")

	// when
	_, err := config.Load(path)

	// then
	require.ErrorIs(t, err, config.ErrInvalidField)
}
