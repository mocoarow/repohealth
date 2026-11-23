package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"path/filepath"

	"github.com/mocoarow/repohealth/internal/config"
	"github.com/mocoarow/repohealth/internal/history"
	"github.com/mocoarow/repohealth/internal/snapshot"
)

func (r runner) runSnapshot(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	path := fs.String("config", config.FileName, "path to the configuration file")

	if done, err := parseFlags(fs, args, stdout, stderr); done {
		return err
	}

	cfg, err := config.Load(*path)
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

	summary, detail, err := snapshot.Take(ctx, snapshot.Input{
		Config:    cfg,
		TakenAt:   r.now(),
		Version:   r.version,
		OutputDir: history.Dir,
	})
	if err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

	if err := history.Save(cfg.Root, summary, detail); err != nil {
		return fmt.Errorf("snapshot: %w", err)
	}

	for _, c := range summary.CoverageReports {
		if c.Skipped == 0 {
			continue
		}

		if _, err := fmt.Fprintf(stderr, "warning: %s: %d source records outside the config directory skipped\n", c.File, c.Skipped); err != nil {
			return fmt.Errorf("write warning: %w", err)
		}
	}

	if _, err := fmt.Fprintf(stdout, "%s: saved to %s\n", summary.Rev, filepath.Join(cfg.Root, history.Dir)); err != nil {
		return fmt.Errorf("write result: %w", err)
	}

	return nil
}
