package config_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/config"
)

func Test_Load_shouldReturnErrInvalidName_whenNameHasDisallowedCharacters(t *testing.T) {
	t.Parallel()

	names := []string{"a>b", "Auth", "-x", "a b", "認証"}
	kinds := map[string]string{
		"area":  "version: 1\nareas: [{name: %q, paths: [auth]}, {name: web, paths: [web]}]\n",
		"group": twoAreas + "groups: [{name: %q, areas: [auth, web]}]\n",
	}

	type testCase struct {
		name string
		yml  string
	}

	tests := make([]testCase, 0, len(names)*len(kinds))

	for kind, format := range kinds {
		for _, n := range names {
			tests = append(tests, testCase{name: kind + " " + n, yml: fmt.Sprintf(format, n)})
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			path := writeRepo(t, tt.yml, "auth", "web")

			// when
			_, err := config.Load(path)

			// then
			require.ErrorIs(t, err, config.ErrInvalidName)
			require.ErrorIs(t, err, config.ErrInvalidConfig)
		})
	}
}

func Test_Load_shouldReturnConfig_whenNameUsesAllowedCharacters(t *testing.T) {
	t.Parallel()

	// given
	yml := "version: 1\nareas: [{name: svc-1_a, paths: [auth]}, {name: \"0web\", paths: [web]}]\ngroups: [{name: back_end-2, areas: [svc-1_a, \"0web\"]}]\n"
	path := writeRepo(t, yml, "auth", "web")

	// when
	_, err := config.Load(path)

	// then
	require.NoError(t, err)
}
