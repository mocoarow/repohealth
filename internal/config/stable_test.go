package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/config"
)

func Test_Load_shouldReturnStablePackages_whenPatternsAreValid(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, twoAreas+"stablePackages: [\"*/domain\", \"*/domain/**\"]\n", "auth", "web")

	// when
	cfg, err := config.Load(path)

	// then
	require.NoError(t, err)
	assert.Equal(t, []string{"*/domain", "*/domain/**"}, cfg.StablePackages)
}

func Test_Load_shouldReturnErrInvalidPattern_whenStablePackageIsMalformed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pattern string
	}{
		{name: "unclosed bracket", pattern: "auth/[domain"},
		{name: "dot slash", pattern: "./auth/domain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			path := writeRepo(t, twoAreas+"stablePackages: [\""+tt.pattern+"\"]\n", "auth", "web")

			// when
			_, err := config.Load(path)

			// then
			require.ErrorIs(t, err, config.ErrInvalidPattern)
		})
	}
}

func Test_Load_shouldReturnErrInvalidField_whenStablePackageIsEmpty(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, twoAreas+"stablePackages: [\"\"]\n", "auth", "web")

	// when
	_, err := config.Load(path)

	// then
	require.ErrorIs(t, err, config.ErrInvalidField)
}

func Test_Config_Stable_shouldReportTrue_whenDirectoryMatchesPattern(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dir  string
	}{
		{name: "package itself", dir: "svc-auth/domain"},
		{name: "sub package", dir: "svc-auth/domain/user"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			cfg := config.Config{StablePackages: []string{"*/domain/**"}}

			// when
			got := cfg.Stable(tt.dir)

			// then
			assert.True(t, got)
		})
	}
}

func Test_Config_Stable_shouldReportFalse_whenDirectoryDoesNotMatch(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		dir  string
	}{
		{name: "other layer", dir: "svc-auth/gateway"},
		{name: "nested deeper", dir: "svc-auth/internal/domain"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			cfg := config.Config{StablePackages: []string{"*/domain/**"}}

			// when
			got := cfg.Stable(tt.dir)

			// then
			assert.False(t, got)
		})
	}
}
