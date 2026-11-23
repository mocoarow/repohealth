package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/mocoarow/repohealth/internal/cli"
)

const (
	exitFailure = 1
	exitUsage   = 2
)

func main() {
	err := cli.Run(context.Background(), os.Args[1:], os.Stdout, os.Stderr)
	if err == nil {
		return
	}

	// The error is the command's answer to a person, so it is printed as text rather than logged.
	_, _ = fmt.Fprintln(os.Stderr, "repohealth:", err)

	if errors.Is(err, cli.ErrUsage) {
		os.Exit(exitUsage)
	}

	os.Exit(exitFailure)
}
