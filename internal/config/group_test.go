package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/config"
)

const twoAreas = "version: 1\nareas: [{name: auth, paths: [auth]}, {name: web, paths: [web]}]\n"

func Test_Load_shouldReturnGroups_whenGroupsAreValid(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, twoAreas+"groups: [{name: backend, areas: [auth, web]}]\n", "auth", "web")

	// when
	cfg, err := config.Load(path)

	// then
	require.NoError(t, err)
	assert.Equal(t, []config.Group{{Name: "backend", Areas: []string{"auth", "web"}}}, cfg.Groups)
}

func Test_Load_shouldReturnErrInvalidField_whenGroupIsInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		groups string
	}{
		{name: "name missing", groups: "groups: [{areas: [auth, web]}]\n"},
		{name: "name duplicated", groups: "groups: [{name: g, areas: [auth, web]}, {name: g, areas: [auth, web]}]\n"},
		{name: "single area", groups: "groups: [{name: g, areas: [auth]}]\n"},
		{name: "area duplicated", groups: "groups: [{name: g, areas: [auth, auth]}]\n"},
		{name: "area empty", groups: "groups: [{name: g, areas: [auth, \"\"]}]\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			path := writeRepo(t, twoAreas+tt.groups, "auth", "web")

			// when
			_, err := config.Load(path)

			// then
			require.ErrorIs(t, err, config.ErrInvalidField)
		})
	}
}

func Test_Load_shouldReturnErrUnknownGroupArea_whenGroupNamesMissingArea(t *testing.T) {
	t.Parallel()

	// given
	path := writeRepo(t, twoAreas+"groups: [{name: backend, areas: [auth, api]}]\n", "auth", "web")

	// when
	_, err := config.Load(path)

	// then
	require.ErrorIs(t, err, config.ErrUnknownGroupArea)
}
