package report

import (
	"encoding/csv"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mocoarow/repohealth/internal/snapshot"
)

const (
	dirPerm  os.FileMode = 0o750
	filePerm os.FileMode = 0o644

	percentScale     = 100
	pctDecimals      = 1
	instabilDecimals = 3
)

// Write writes the history and the tables of the latest detail as CSV files into dir, creating dir and
// overwriting the files that exist.
func Write(dir string, summaries []snapshot.Summary, latest snapshot.Detail) error {
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	if err := writeHistory(dir, summaries); err != nil {
		return err
	}

	return writeLatest(dir, latest)
}

func writeCSV(dir, name string, header []string, rows [][]string) error {
	path := filepath.Join(dir, name)

	return writeFile(path, func(f io.Writer) error {
		w := csv.NewWriter(f)
		if err := w.Write(header); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}

		if err := w.WriteAll(rows); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}

		w.Flush()

		if err := w.Error(); err != nil {
			return fmt.Errorf("flush %s: %w", path, err)
		}

		return nil
	})
}

func writeFile(path string, write func(io.Writer) error) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, filePerm) //nolint:gosec // the caller chose the path
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}

	defer func() {
		if cerr := f.Close(); cerr != nil {
			slog.Error("close file", "path", path, "error", cerr)

			if err == nil {
				err = fmt.Errorf("close %s: %w", path, cerr)
			}
		}
	}()

	return write(f)
}

func itoa(n int) string {
	return strconv.Itoa(n)
}

func formatTime(t time.Time) string {
	return t.Format(time.RFC3339)
}

// percent reports covered/lines as a percentage, and false when there are no lines.
func percent(covered, lines int) (string, bool) {
	if lines == 0 {
		return "", false
	}

	return formatFloat(float64(covered)*percentScale/float64(lines), pctDecimals), true
}

func formatFloat(v float64, decimals int) string {
	return strconv.FormatFloat(v, 'f', decimals, 64)
}
