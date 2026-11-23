package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/mocoarow/repohealth/internal/snapshot"
)

const (
	// Dir is the directory, relative to the config directory, that holds the snapshot files.
	Dir = ".repohealth"

	historyName = "history.jsonl"
	latestName  = "latest.json"
	tempPattern = "latest-*.json"

	dirPerm  os.FileMode = 0o750
	filePerm os.FileMode = 0o644
	newline              = '\n'
	indent               = "  "
)

var (
	// ErrMalformedHistory reports a history line that is blank, truncated or not JSON.
	ErrMalformedHistory = errors.New("malformed history")
	// ErrUnknownSchema reports a snapshot written with a schemaVersion this version does not know.
	ErrUnknownSchema = errors.New("unknown schema version")
)

// Save appends s to the history and replaces the latest detail with d, creating the directory when needed.
// It writes nothing when the history does not end with a newline.
func Save(root string, s snapshot.Summary, d snapshot.Detail) error {
	dir := filepath.Join(root, Dir)
	historyPath := filepath.Join(dir, historyName)

	line, err := json.Marshal(s)
	if err != nil {
		return fmt.Errorf("encode summary: %w", err)
	}

	latest, err := json.MarshalIndent(d, "", indent)
	if err != nil {
		return fmt.Errorf("encode detail: %w", err)
	}

	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	if err := checkTail(historyPath); err != nil {
		return err
	}

	temp, err := writeTemp(dir, append(latest, newline))
	if err != nil {
		return err
	}

	if err := appendLine(historyPath, append(line, newline)); err != nil {
		removeTemp(temp)

		return err
	}

	if err := os.Rename(temp, filepath.Join(dir, latestName)); err != nil {
		removeTemp(temp)

		return fmt.Errorf("replace %s: %w", latestName, err)
	}

	return nil
}

// checkTail rejects a history whose last line has no newline, so an append cannot join two records.
func checkTail(path string) (err error) {
	f, err := os.Open(path) //nolint:gosec // the path is inside the repository root the caller chose
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}

	defer closeFile(f, &err)

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	if info.Size() == 0 {
		return nil
	}

	last := make([]byte, 1)
	if _, err := f.ReadAt(last, info.Size()-1); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	if last[0] != newline {
		return fmt.Errorf("%w: %s does not end with a newline", ErrMalformedHistory, historyName)
	}

	return nil
}

func writeTemp(dir string, content []byte) (name string, err error) {
	f, err := os.CreateTemp(dir, tempPattern)
	if err != nil {
		return "", fmt.Errorf("create temp file in %s: %w", dir, err)
	}

	name = f.Name()

	defer func() {
		if err != nil {
			removeTemp(name)
		}
	}()

	defer closeFile(f, &err)

	if _, err := f.Write(content); err != nil {
		return "", fmt.Errorf("write %s: %w", name, err)
	}

	if err := f.Chmod(filePerm); err != nil {
		return "", fmt.Errorf("chmod %s: %w", name, err)
	}

	return name, nil
}

func appendLine(path string, line []byte) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, filePerm) //nolint:gosec // the path is inside the repository root the caller chose
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}

	defer closeFile(f, &err)

	if _, err := f.Write(line); err != nil {
		return fmt.Errorf("append to %s: %w", path, err)
	}

	return nil
}

// closeFile closes f and reports a failure to close as the result when nothing else failed, since buffered data may be lost.
func closeFile(f *os.File, err *error) {
	if cerr := f.Close(); cerr != nil {
		slog.Error("close file", "path", f.Name(), "error", cerr)

		if *err == nil {
			*err = fmt.Errorf("close %s: %w", f.Name(), cerr)
		}
	}
}

func removeTemp(path string) {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Error("remove temp file", "path", path, "error", err)
	}
}
