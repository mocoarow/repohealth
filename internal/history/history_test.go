package history_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mocoarow/repohealth/internal/history"
)

const (
	historyFile = "history.jsonl"
	latestFile  = "latest.json"
	filePerm    = 0o600
	longRevLen  = 100 * 1024
	truncated   = `{"schemaVersion":1,"ta`
)

func readFile(t *testing.T, root string, name string) []byte {
	t.Helper()

	b, err := os.ReadFile(filepath.Join(root, history.Dir, name)) //nolint:gosec // the path is inside a test directory
	require.NoError(t, err)

	return b
}

func writeFile(t *testing.T, root string, name string, content string) {
	t.Helper()

	dir := filepath.Join(root, history.Dir)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), filePerm))
}

func save(t *testing.T, root string, rev string) {
	t.Helper()

	require.NoError(t, history.Save(root, summaryOf(t, rev), detailOf(t, rev)))
}

func Test_Save_shouldEndHistoryWithNewline_whenHistoryExists(t *testing.T) {
	t.Parallel()

	// given
	root := t.TempDir()
	save(t, root, "first")

	// when
	save(t, root, "second")

	// then
	assert.True(t, bytes.HasSuffix(readFile(t, root, historyFile), []byte("\n")))
}

func Test_Save_shouldWriteOneLinePerSave_whenHistoryExists(t *testing.T) {
	t.Parallel()

	// given
	root := t.TempDir()
	save(t, root, "first")

	// when
	save(t, root, "second")

	// then
	lines := strings.Split(strings.TrimSuffix(string(readFile(t, root, historyFile)), "\n"), "\n")
	assert.Len(t, lines, 2)
}

func Test_Save_shouldKeepEarlierLineFirst_whenHistoryExists(t *testing.T) {
	t.Parallel()

	// given
	root := t.TempDir()
	save(t, root, "first")

	// when
	save(t, root, "second")

	// then
	lines := strings.Split(strings.TrimSuffix(string(readFile(t, root, historyFile)), "\n"), "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], `"rev":"first"`)
}

func Test_Save_shouldAppendNewLineLast_whenHistoryExists(t *testing.T) {
	t.Parallel()

	// given
	root := t.TempDir()
	save(t, root, "first")

	// when
	save(t, root, "second")

	// then
	lines := strings.Split(strings.TrimSuffix(string(readFile(t, root, historyFile)), "\n"), "\n")
	require.Len(t, lines, 2)
	assert.Contains(t, lines[1], `"rev":"second"`)
}

func Test_Save_shouldReplaceLatest_whenLatestExists(t *testing.T) {
	t.Parallel()

	// given
	root := t.TempDir()
	save(t, root, "first")

	// when
	save(t, root, "second")

	// then
	b := readFile(t, root, latestFile)
	assert.Contains(t, string(b), `"rev": "second"`)
	assert.NotContains(t, string(b), `"first"`)
}

func Test_Save_shouldLeaveOnlyTwoFiles_whenSaved(t *testing.T) {
	t.Parallel()

	// given
	root := t.TempDir()

	// when
	save(t, root, "first")
	save(t, root, "second")

	// then
	entries, err := os.ReadDir(filepath.Join(root, history.Dir))
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	assert.ElementsMatch(t, []string{historyFile, latestFile}, names)
}

func Test_Save_shouldReturnErrMalformedHistory_whenLastLineIsTruncated(t *testing.T) {
	t.Parallel()

	// given
	root := t.TempDir()
	writeFile(t, root, historyFile, truncated)

	// when
	err := history.Save(root, summaryOf(t, "rev"), detailOf(t, "rev"))

	// then
	require.ErrorIs(t, err, history.ErrMalformedHistory)
}

func Test_Save_shouldWriteNothing_whenLastLineIsTruncated(t *testing.T) {
	t.Parallel()

	// given
	root := t.TempDir()
	writeFile(t, root, historyFile, truncated)

	// when
	_ = history.Save(root, summaryOf(t, "rev"), detailOf(t, "rev"))

	// then
	entries, err := os.ReadDir(filepath.Join(root, history.Dir))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, truncated, string(readFile(t, root, historyFile)))
}

func Test_Save_shouldMatchGolden_whenInputIsFixed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		golden string
	}{
		{name: historyFile, golden: filepath.Join("testdata", historyFile)},
		{name: latestFile, golden: filepath.Join("testdata", latestFile)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			root := t.TempDir()
			want, err := os.ReadFile(tt.golden)
			require.NoError(t, err)

			// when
			save(t, root, "abc123def456")

			// then
			assert.Equal(t, string(want), string(readFile(t, root, tt.name)))
		})
	}
}

func savedTwice(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	save(t, root, "first")
	save(t, root, "second")

	return root
}

func Test_Load_shouldReturnSummariesInSaveOrder_whenSavedTwice(t *testing.T) {
	t.Parallel()

	// given
	root := savedTwice(t)

	// when
	summaries, _, err := history.Load(root)

	// then
	require.NoError(t, err)
	require.Len(t, summaries, 2)
	assert.Equal(t, []string{"first", "second"}, []string{summaries[0].Rev, summaries[1].Rev})
}

func Test_Load_shouldReturnFirstSummary_whenSavedTwice(t *testing.T) {
	t.Parallel()

	// given
	root := savedTwice(t)

	// when
	summaries, _, err := history.Load(root)

	// then
	require.NoError(t, err)
	require.Len(t, summaries, 2)
	assert.Equal(t, summaryOf(t, "first"), summaries[0])
}

func Test_Load_shouldReturnSecondSummary_whenSavedTwice(t *testing.T) {
	t.Parallel()

	// given
	root := savedTwice(t)

	// when
	summaries, _, err := history.Load(root)

	// then
	require.NoError(t, err)
	require.Len(t, summaries, 2)
	assert.Equal(t, summaryOf(t, "second"), summaries[1])
}

func Test_Load_shouldReturnLatestDetail_whenSavedTwice(t *testing.T) {
	t.Parallel()

	// given
	root := savedTwice(t)

	// when
	_, detail, err := history.Load(root)

	// then
	require.NoError(t, err)
	assert.Equal(t, detailOf(t, "second"), detail)
}

func Test_Load_shouldReturnNoSummaries_whenHistoryIsEmpty(t *testing.T) {
	t.Parallel()

	// given
	root := t.TempDir()
	save(t, root, "first")
	writeFile(t, root, historyFile, "")

	// when
	summaries, _, err := history.Load(root)

	// then
	require.NoError(t, err)
	assert.Empty(t, summaries)
}

func Test_Load_shouldReadLine_whenLineExceeds64KiB(t *testing.T) {
	t.Parallel()

	// given
	root := t.TempDir()
	rev := strings.Repeat("a", longRevLen)
	save(t, root, rev)

	// when
	summaries, _, err := history.Load(root)

	// then
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Equal(t, rev, summaries[0].Rev)
}

func Test_Load_shouldReturnErrMalformedHistory_whenLastLineIsTruncated(t *testing.T) {
	t.Parallel()

	// given
	root := t.TempDir()
	save(t, root, "first")
	line := string(readFile(t, root, historyFile))
	writeFile(t, root, historyFile, line+truncated)

	// when
	_, _, err := history.Load(root)

	// then
	require.ErrorIs(t, err, history.ErrMalformedHistory)
}

func Test_Load_shouldReturnErrMalformedHistory_whenLineIsBlank(t *testing.T) {
	t.Parallel()

	// given
	root := t.TempDir()
	save(t, root, "first")
	line := string(readFile(t, root, historyFile))
	writeFile(t, root, historyFile, line+"\n"+line)

	// when
	_, _, err := history.Load(root)

	// then
	require.ErrorIs(t, err, history.ErrMalformedHistory)
}

func Test_Load_shouldNameLineNumber_whenLineIsMalformed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		suffix func(line string) string
	}{
		{name: "truncated", suffix: func(line string) string { return line + truncated }},
		{name: "blank", suffix: func(line string) string { return line + "\n" + line }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			root := t.TempDir()
			save(t, root, "first")
			writeFile(t, root, historyFile, tt.suffix(string(readFile(t, root, historyFile))))

			// when
			_, _, err := history.Load(root)

			// then
			assert.ErrorContains(t, err, "line 2")
		})
	}
}

func Test_Load_shouldReturnErrUnknownSchema_whenSchemaVersionIsUnknown(t *testing.T) {
	t.Parallel()

	const future = `{"schemaVersion":99}` + "\n"

	tests := []struct {
		name string
		file string
	}{
		{name: "history", file: historyFile},
		{name: "latest", file: latestFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			root := t.TempDir()
			save(t, root, "first")
			writeFile(t, root, tt.file, future)

			// when
			_, _, err := history.Load(root)

			// then
			require.ErrorIs(t, err, history.ErrUnknownSchema)
		})
	}
}

func Test_Load_shouldReturnErrNotExist_whenFileIsMissing(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
	}{
		{name: "history", file: historyFile},
		{name: "latest", file: latestFile},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// given
			root := t.TempDir()
			save(t, root, "first")
			require.NoError(t, os.Remove(filepath.Join(root, history.Dir, tt.file)))

			// when
			_, _, err := history.Load(root)

			// then
			require.ErrorIs(t, err, fs.ErrNotExist)
		})
	}
}
