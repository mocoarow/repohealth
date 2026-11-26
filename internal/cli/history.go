package cli

import (
	"flag"
	"fmt"
	"io"

	"github.com/mocoarow/repohealth/internal/config"
	"github.com/mocoarow/repohealth/internal/history"
	"github.com/mocoarow/repohealth/internal/snapshot"
)

// historyOutput is a command that writes the saved history to the path given by -out.
type historyOutput struct {
	name       string
	outHelp    string
	write      func(out string, summaries []snapshot.Summary, latest snapshot.Detail) error
	donePrefix string
}

func (h historyOutput) run(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet(h.name, flag.ContinueOnError)
	path := fs.String("config", config.FileName, "path to the configuration file")
	out := fs.String("out", "", h.outHelp)

	if done, err := parseFlags(fs, args, stdout, stderr); done {
		return err
	}

	if *out == "" {
		return writeFlags(stderr, fs, fmt.Errorf("%w: %s: -out is required", ErrUsage, h.name))
	}

	cfg, err := config.Load(*path)
	if err != nil {
		return fmt.Errorf("%s: %w", h.name, err)
	}

	summaries, latest, err := history.Load(cfg.Root)
	if err != nil {
		return fmt.Errorf("%s: %w", h.name, err)
	}

	if err := h.write(*out, summaries, latest); err != nil {
		return fmt.Errorf("%s: %w", h.name, err)
	}

	if _, err := fmt.Fprintf(stdout, "%s%s\n", h.donePrefix, *out); err != nil {
		return fmt.Errorf("write result: %w", err)
	}

	return nil
}
