package cli_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/cli"
	"github.com/mocoarow/repohealth/internal/config"
)

func writeValidConfig(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "a"), 0o750))

	path := filepath.Join(root, config.FileName)
	require.NoError(t, os.WriteFile(path, []byte("version: 1\nareas: [{name: a, paths: [a]}]\n"), 0o600))

	return path
}

// closedFile returns a writer whose every write fails with os.ErrClosed.
func closedFile(t *testing.T) *os.File {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "out")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	return f
}

func Test_Run_shouldReturnErrClosed_whenOutputCannotBeWritten(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args func(path string) []string
	}{
		{name: "help", args: func(string) []string { return []string{"help"} }},
		{name: "validate help", args: func(string) []string { return []string{"validate", "-h"} }},
		{name: "validate result", args: func(path string) []string { return []string{"validate", "-config", path} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			path := writeValidConfig(t)

			// when
			err := cli.Run(t.Context(), tt.args(path), closedFile(t), &bytes.Buffer{})

			// then
			require.ErrorIs(t, err, os.ErrClosed)
		})
	}
}

func Test_Run_shouldReturnErrUsage_whenUsageCannotBeWritten(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "missing command", args: nil},
		{name: "unknown command", args: []string{"snapshto"}},
		{name: "unknown validate flag", args: []string{"validate", "-unknown"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			stderr := closedFile(t)

			// when
			err := cli.Run(t.Context(), tt.args, &bytes.Buffer{}, stderr)

			// then
			require.ErrorIs(t, err, cli.ErrUsage)
		})
	}
}

func Test_Run_shouldPrintFlagsToStderr_whenValidateArgumentsAreInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		extra []string
	}{
		{name: "unknown flag", extra: []string{"-unknown"}},
		{name: "extra argument", extra: []string{"extra"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			path := writeValidConfig(t)

			var errOut bytes.Buffer

			// when
			_ = cli.Run(t.Context(), append([]string{"validate", "-config", path}, tt.extra...), &bytes.Buffer{}, &errOut)

			// then
			assert.Contains(t, errOut.String(), "-config")
		})
	}
}

func Test_Run_shouldPrintOK_whenConfigIsValid(t *testing.T) {
	t.Parallel()

	// given
	path := writeValidConfig(t)

	var out bytes.Buffer

	// when
	err := cli.Run(t.Context(), []string{"validate", "-config", path}, &out, &bytes.Buffer{})

	// then
	require.NoError(t, err)
	assert.Equal(t, path+": ok\n", out.String())
}

func Test_Run_shouldReturnErrInvalidConfig_whenConfigIsInvalid(t *testing.T) {
	t.Parallel()

	// given
	path := filepath.Join(t.TempDir(), config.FileName)
	require.NoError(t, os.WriteFile(path, []byte("version: 2\n"), 0o600))

	// when
	err := cli.Run(t.Context(), []string{"validate", "-config", path}, &bytes.Buffer{}, &bytes.Buffer{})

	// then
	require.ErrorIs(t, err, config.ErrInvalidConfig)
}

func Test_Run_shouldReturnErrNotExist_whenConfigIsMissing(t *testing.T) {
	t.Parallel()

	// given
	path := filepath.Join(t.TempDir(), config.FileName)

	// when
	err := cli.Run(t.Context(), []string{"validate", "-config", path}, &bytes.Buffer{}, &bytes.Buffer{})

	// then
	require.ErrorIs(t, err, fs.ErrNotExist)
}

func Test_Run_shouldReturnErrUsage_whenValidateArgumentsAreInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		extra []string
	}{
		{name: "unknown flag", extra: []string{"-unknown"}},
		{name: "extra argument", extra: []string{"extra"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			path := writeValidConfig(t)

			// when
			err := cli.Run(t.Context(), append([]string{"validate", "-config", path}, tt.extra...), &bytes.Buffer{}, &bytes.Buffer{})

			// then
			require.ErrorIs(t, err, cli.ErrUsage)
		})
	}
}

func Test_Run_shouldPrintFlags_whenValidateHelpIsRequested(t *testing.T) {
	t.Parallel()

	// given
	var out bytes.Buffer

	// when
	err := cli.Run(t.Context(), []string{"validate", "-h"}, &out, &bytes.Buffer{})

	// then
	require.NoError(t, err)
	assert.Contains(t, out.String(), "-config")
}

func Test_Run_shouldPrintCommandsToStdout_whenHelpIsRequested(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		arg  string
	}{
		{name: "-h", arg: "-h"},
		{name: "-help", arg: "-help"},
		{name: "--help", arg: "--help"},
		{name: "help", arg: "help"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			var out bytes.Buffer

			// when
			err := cli.Run(t.Context(), []string{tt.arg}, &out, &bytes.Buffer{})

			// then
			require.NoError(t, err)
			assert.Contains(t, out.String(), "validate")
		})
	}
}

func Test_Run_shouldReturnErrUsage_whenCommandIsMissing(t *testing.T) {
	t.Parallel()

	// given
	var args []string

	// when
	err := cli.Run(t.Context(), args, &bytes.Buffer{}, &bytes.Buffer{})

	// then
	require.ErrorIs(t, err, cli.ErrUsage)
}

func Test_Run_shouldPrintCommandsToStderr_whenCommandIsMissing(t *testing.T) {
	t.Parallel()

	// given
	var errOut bytes.Buffer

	// when
	_ = cli.Run(t.Context(), nil, &bytes.Buffer{}, &errOut)

	// then
	assert.Contains(t, errOut.String(), "validate")
}

func Test_Run_shouldPrintCommandsToStderr_whenCommandIsUnknown(t *testing.T) {
	t.Parallel()

	// given
	var errOut bytes.Buffer

	// when
	_ = cli.Run(t.Context(), []string{"snapshto"}, &bytes.Buffer{}, &errOut)

	// then
	assert.Contains(t, errOut.String(), "validate")
}

func Test_Run_shouldReturnErrUsage_whenCommandIsUnknown(t *testing.T) {
	t.Parallel()

	// given
	args := []string{"snapshto"}

	// when
	err := cli.Run(t.Context(), args, &bytes.Buffer{}, &bytes.Buffer{})

	// then
	require.ErrorIs(t, err, cli.ErrUsage)
}

func Test_Run_shouldReturnErrUnknownCommand_whenCommandIsUnknown(t *testing.T) {
	t.Parallel()

	// given
	args := []string{"snapshto"}

	// when
	err := cli.Run(t.Context(), args, &bytes.Buffer{}, &bytes.Buffer{})

	// then
	require.ErrorIs(t, err, cli.ErrUnknownCommand)
}
