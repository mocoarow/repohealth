package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/cli"
	"github.com/mocoarow/repohealth/internal/config"
)

func Test_Run_shouldWriteMarkdown_whenSnapshotExists(t *testing.T) {
	t.Parallel()

	// given
	root := newGoRepo(t, nil)
	require.NoError(t, cli.RunAt(t.Context(), takenAt, "v1", snapshotArgs(root), &bytes.Buffer{}, &bytes.Buffer{}))

	out := filepath.Join(t.TempDir(), "REPORT.md")
	args := []string{"markdown", "-config", filepath.Join(root, config.FileName), "-out", out}

	// when
	err := cli.Run(t.Context(), args, &bytes.Buffer{}, &bytes.Buffer{})

	// then
	require.NoError(t, err)
	assert.FileExists(t, out)
}

func Test_Run_shouldPrintOutputFile_whenMarkdownIsWritten(t *testing.T) {
	t.Parallel()

	// given
	root := newGoRepo(t, nil)
	require.NoError(t, cli.RunAt(t.Context(), takenAt, "v1", snapshotArgs(root), &bytes.Buffer{}, &bytes.Buffer{}))

	out := filepath.Join(t.TempDir(), "REPORT.md")
	args := []string{"markdown", "-config", filepath.Join(root, config.FileName), "-out", out}
	var stdout bytes.Buffer

	// when
	err := cli.Run(t.Context(), args, &stdout, &bytes.Buffer{})

	// then
	require.NoError(t, err)
	assert.Equal(t, "wrote "+out+"\n", stdout.String())
}

func Test_Run_shouldReturnErrNotExist_whenMarkdownHasNoHistory(t *testing.T) {
	t.Parallel()

	// given
	root := newGoRepo(t, nil)
	args := []string{"markdown", "-config", filepath.Join(root, config.FileName), "-out", filepath.Join(t.TempDir(), "REPORT.md")}

	// when
	err := cli.Run(t.Context(), args, &bytes.Buffer{}, &bytes.Buffer{})

	// then
	require.ErrorIs(t, err, os.ErrNotExist)
}

func Test_Run_shouldReturnErrUsage_whenMarkdownArgumentsAreInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "out is missing", args: []string{"markdown"}},
		{name: "unknown flag", args: []string{"markdown", "-nope"}},
		{name: "positional argument", args: []string{"markdown", "-out", "x", "extra"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			var stderr bytes.Buffer

			// when
			err := cli.Run(t.Context(), tt.args, &bytes.Buffer{}, &stderr)

			// then
			require.ErrorIs(t, err, cli.ErrUsage)
			assert.Contains(t, stderr.String(), "usage: repohealth markdown")
		})
	}
}

func Test_Run_shouldPrintFlags_whenMarkdownHelpIsRequested(t *testing.T) {
	t.Parallel()

	// given
	var stdout bytes.Buffer

	// when
	err := cli.Run(t.Context(), []string{"markdown", "-h"}, &stdout, &bytes.Buffer{})

	// then
	require.NoError(t, err)
	assert.Contains(t, stdout.String(), "-out")
}
