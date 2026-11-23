package cli

import (
	"context"
	"flag"
	"fmt"
	"io"

	"github.com/mocoarow/repohealth/internal/config"
	"github.com/mocoarow/repohealth/internal/history"
	"github.com/mocoarow/repohealth/internal/report"
)

func runReport(_ context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	path := fs.String("config", config.FileName, "path to the configuration file")
	out := fs.String("out", "", "directory to write the CSV files to (required)")

	if done, err := parseFlags(fs, args, stdout, stderr); done {
		return err
	}

	if *out == "" {
		return writeFlags(stderr, fs, fmt.Errorf("%w: report: -out is required", ErrUsage))
	}

	cfg, err := config.Load(*path)
	if err != nil {
		return fmt.Errorf("report: %w", err)
	}

	summaries, latest, err := history.Load(cfg.Root)
	if err != nil {
		return fmt.Errorf("report: %w", err)
	}

	if err := report.Write(*out, summaries, latest); err != nil {
		return fmt.Errorf("report: %w", err)
	}

	if _, err := fmt.Fprintf(stdout, "wrote CSV files to %s\n", *out); err != nil {
		return fmt.Errorf("write result: %w", err)
	}

	return nil
}
