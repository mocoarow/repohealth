package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

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

func commands() []command {
	return []command{
		{name: "validate", summary: "load and validate the configuration file", run: runValidate},
	}
}

// Run executes the command named by args[0]. Results go to stdout; usage after a mistake goes to stderr.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return writeUsage(stderr, ErrUsage)
	}

	switch args[0] {
	case "-h", "-help", "--help", "help":
		return writeUsage(stdout, nil)
	}

	for _, c := range commands() {
		if c.name == args[0] {
			return c.run(ctx, args[1:], stdout, stderr)
		}
	}

	return writeUsage(stderr, fmt.Errorf("%w: %w: %s", ErrUsage, ErrUnknownCommand, args[0]))
}

// writeUsage writes the command list to w and then returns cause.
func writeUsage(w io.Writer, cause error) error {
	var b strings.Builder

	b.WriteString("usage: repohealth <command> [flags]\n\ncommands:\n")

	for _, c := range commands() {
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

func runValidate(_ context.Context, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	path := fs.String("config", config.FileName, "path to the configuration file")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return writeFlags(stdout, fs, nil)
		}

		return writeFlags(stderr, fs, fmt.Errorf("%w: validate: %w", ErrUsage, err))
	}

	if fs.NArg() > 0 {
		return writeFlags(stderr, fs, fmt.Errorf("%w: validate: unexpected argument %s", ErrUsage, fs.Arg(0)))
	}

	if _, err := config.Load(*path); err != nil {
		return fmt.Errorf("validate: %w", err)
	}

	if _, err := fmt.Fprintf(stdout, "%s: ok\n", *path); err != nil {
		return fmt.Errorf("write result: %w", err)
	}

	return nil
}
