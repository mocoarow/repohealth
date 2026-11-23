package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime/debug"
	"strings"
	"time"

	"github.com/mocoarow/repohealth/internal/config"
)

var (
	// ErrUsage reports arguments that the command line does not accept.
	ErrUsage = errors.New("invalid usage")
	// ErrUnknownCommand reports a command name this build does not provide; it is returned together with ErrUsage.
	ErrUnknownCommand = errors.New("unknown command")
)

type command struct {
	name    string
	summary string
	run     func(ctx context.Context, args []string, stdout, stderr io.Writer) error
}

// runner holds what commands read from the environment, so tests can fix it.
type runner struct {
	now     func() time.Time
	version string
}

func newRunner() runner {
	version := ""
	if info, ok := debug.ReadBuildInfo(); ok {
		version = info.Main.Version
	}

	return runner{now: time.Now, version: version}
}

func (r runner) commands() []command {
	return []command{
		{name: "validate", summary: "load and validate the configuration file", run: runValidate},
		{name: "snapshot", summary: "measure HEAD and append the result to .repohealth", run: r.runSnapshot},
		{name: "report", summary: "write CSV files from the saved history", run: runReport},
	}
}

// Run executes the command named by args[0]. Results go to stdout; usage after a mistake goes to stderr.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	return newRunner().run(ctx, args, stdout, stderr)
}

func (r runner) run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return writeUsage(stderr, r, ErrUsage)
	}

	switch args[0] {
	case "-h", "-help", "--help", "help":
		return writeUsage(stdout, r, nil)
	}

	for _, c := range r.commands() {
		if c.name == args[0] {
			return c.run(ctx, args[1:], stdout, stderr)
		}
	}

	return writeUsage(stderr, r, fmt.Errorf("%w: %w: %s", ErrUsage, ErrUnknownCommand, args[0]))
}

// writeUsage writes the command list to w and then returns cause.
func writeUsage(w io.Writer, r runner, cause error) error {
	var b strings.Builder

	b.WriteString("usage: repohealth <command> [flags]\n\ncommands:\n")

	for _, c := range r.commands() {
		fmt.Fprintf(&b, "  %-10s %s\n", c.name, c.summary)
	}

	return writeText(w, b.String(), cause)
}

// writeFlags writes the flags of fs to w and then returns cause.
func writeFlags(w io.Writer, fs *flag.FlagSet, cause error) error {
	var b strings.Builder

	fmt.Fprintf(&b, "usage: repohealth %s [flags]\n\nflags:\n", fs.Name())
	fs.SetOutput(&b)
	fs.PrintDefaults()

	return writeText(w, b.String(), cause)
}

func writeText(w io.Writer, text string, cause error) error {
	if _, err := io.WriteString(w, text); err != nil {
		return fmt.Errorf("write usage: %w", errors.Join(err, cause))
	}

	return cause
}

// parseFlags parses args into fs. done is true when the command must stop: help was shown or the arguments were wrong.
func parseFlags(fs *flag.FlagSet, args []string, stdout, stderr io.Writer) (done bool, err error) {
	fs.SetOutput(io.Discard)

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return true, writeFlags(stdout, fs, nil)
		}

		return true, writeFlags(stderr, fs, fmt.Errorf("%w: %s: %w", ErrUsage, fs.Name(), err))
	}

	if fs.NArg() > 0 {
		return true, writeFlags(stderr, fs, fmt.Errorf("%w: %s: unexpected argument %s", ErrUsage, fs.Name(), fs.Arg(0)))
	}

	return false, nil
}

func runValidate(_ context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	path := fs.String("config", config.FileName, "path to the configuration file")

	if done, err := parseFlags(fs, args, stdout, stderr); done {
		return err
	}

	if _, err := config.Load(*path); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	if _, err := fmt.Fprintf(stdout, "%s: ok\n", *path); err != nil {
		return fmt.Errorf("write result: %w", err)
	}

	return nil
}
