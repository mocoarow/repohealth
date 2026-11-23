package gosrc_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mocoarow/repohealth/internal/config"
	"github.com/mocoarow/repohealth/internal/gosrc"
)

func testConfig() config.Config {
	return config.Config{
		Root:           "/repo",
		Version:        1,
		Areas:          []config.Area{{Name: "svc", Paths: []string{"svc"}}},
		Groups:         nil,
		Exclude:        []string{"svc/gen/**"},
		StablePackages: nil,
	}
}

func Test_Counted_shouldReportTrue_whenFileIsGoSourceInArea(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
	}{
		{name: "package file", file: "svc/domain/user.go"},
		{name: "file at area root", file: "svc/main.go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			cfg := testConfig()

			// when
			got := gosrc.Counted(cfg, tt.file)

			// then
			assert.True(t, got)
		})
	}
}

func Test_Counted_shouldReportFalse_whenFileIsNotCounted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
	}{
		{name: "test file", file: "svc/domain/user_test.go"},
		{name: "not go", file: "svc/domain/user.ts"},
		{name: "excluded", file: "svc/gen/api.go"},
		{name: "outside every area", file: "tools/main.go"},
		{name: "testdata", file: "svc/domain/testdata/broken.go"},
		{name: "dot directory", file: "svc/.cache/x.go"},
		{name: "underscore directory", file: "svc/_old/x.go"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			cfg := testConfig()

			// when
			got := gosrc.Counted(cfg, tt.file)

			// then
			assert.False(t, got)
		})
	}
}
