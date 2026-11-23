package history

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/mocoarow/repohealth/internal/snapshot"
)

// Load reads every summary of the history and the latest detail.
// A missing file is reported as an error wrapping fs.ErrNotExist.
func Load(root string) ([]snapshot.Summary, snapshot.Detail, error) {
	dir := filepath.Join(root, Dir)

	summaries, err := loadHistory(filepath.Join(dir, historyName))
	if err != nil {
		return nil, snapshot.Detail{}, err
	}

	var d snapshot.Detail
	if err := loadLatest(filepath.Join(dir, latestName), &d); err != nil {
		return nil, snapshot.Detail{}, err
	}

	return summaries, d, nil
}

func loadHistory(path string) (summaries []snapshot.Summary, err error) {
	f, err := os.Open(path) //nolint:gosec // the path is inside the repository root the caller chose
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}

	defer closeFile(f, &err)

	r := bufio.NewReader(f)

	for n := 1; ; n++ {
		line, err := r.ReadBytes(newline)
		if errors.Is(err, io.EOF) && len(line) == 0 {
			return summaries, nil
		}

		if err != nil {
			return nil, fmt.Errorf("%w: line %d of %s is not terminated by a newline", ErrMalformedHistory, n, historyName)
		}

		var s snapshot.Summary
		if err := decode(line, &s); err != nil {
			return nil, fmt.Errorf("line %d of %s: %w", n, historyName, err)
		}

		summaries = append(summaries, s)
	}
}

func loadLatest(path string, d *snapshot.Detail) error {
	b, err := os.ReadFile(path) //nolint:gosec // the path is inside the repository root the caller chose
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	if err := decode(b, d); err != nil {
		return fmt.Errorf("%s: %w", latestName, err)
	}

	return nil
}

// decode checks the schema version before reading the rest, so a newer layout is not reported as malformed.
func decode(b []byte, v any) error {
	var h snapshot.Header
	if err := json.Unmarshal(b, &h); err != nil {
		return fmt.Errorf("%w: %w", ErrMalformedHistory, err)
	}

	if h.SchemaVersion != snapshot.SchemaVersion {
		return fmt.Errorf("%w: %d", ErrUnknownSchema, h.SchemaVersion)
	}

	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%w: %w", ErrMalformedHistory, err)
	}

	return nil
}
