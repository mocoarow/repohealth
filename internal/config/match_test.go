package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mocoarow/repohealth/internal/config"
)

func matchConfig() config.Config {
	return config.Config{
		Root:    "/repo",
		Version: 1,
		Areas: []config.Area{
			{Name: "auth", Paths: []string{"svc-auth"}},
			{Name: "web", Paths: []string{"svc-web", "shared/ui"}},
		},
		Groups:  nil,
		Exclude: []string{"**/*_mock.go", "svc-web/build/**"},
	}
}

func Test_Config_AreaOf_shouldReturnArea_whenFileIsUnderAreaPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
		want string
	}{
		{name: "first path", file: "svc-auth/domain/user.go", want: "auth"},
		{name: "second path", file: "shared/ui/button.tsx", want: "web"},
		{name: "directly under path", file: "svc-web/package.json", want: "web"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			cfg := matchConfig()

			// when
			got, _ := cfg.AreaOf(tt.file)

			// then
			assert.Equal(t, tt.want, got)
		})
	}
}

func Test_Config_AreaOf_shouldReportFalse_whenFileIsNotCounted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
	}{
		{name: "outside every area", file: "docs/readme.md"},
		{name: "shares name prefix", file: "svc-authz/main.go"},
		{name: "excluded by suffix", file: "svc-auth/gateway/repo_mock.go"},
		{name: "excluded by directory", file: "svc-web/build/index.js"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			cfg := matchConfig()

			// when
			_, ok := cfg.AreaOf(tt.file)

			// then
			assert.False(t, ok)
		})
	}
}

func Test_Config_PathOf_shouldReturnAreaPath_whenFileIsUnderAreaPath(t *testing.T) {
	t.Parallel()

	// given
	cfg := matchConfig()

	// when
	got, _ := cfg.PathOf("shared/ui/button.tsx")

	// then
	assert.Equal(t, "shared/ui", got)
}

func Test_Config_PathOf_shouldReportFalse_whenFileIsExcluded(t *testing.T) {
	t.Parallel()

	// given
	cfg := matchConfig()

	// when
	_, ok := cfg.PathOf("svc-web/build/index.js")

	// then
	assert.False(t, ok)
}

func Test_Config_AreaOf_shouldReturnArea_whenAreaIsRepositoryRoot(t *testing.T) {
	t.Parallel()

	// given
	cfg := config.Config{Root: "/repo", Version: 1, Areas: []config.Area{{Name: "all", Paths: []string{"."}}}, Groups: nil, Exclude: nil}

	// when
	got, _ := cfg.AreaOf("main.go")

	// then
	assert.Equal(t, "all", got)
}
