package cli

import (
	"context"
	"io"

	"github.com/mocoarow/repohealth/internal/report"
)

func runReport(_ context.Context, args []string, stdout, stderr io.Writer) error {
	return historyOutput{
		name:       "report",
		outHelp:    "directory to write the CSV files to (required)",
		write:      report.Write,
		donePrefix: "wrote CSV files to ",
	}.run(args, stdout, stderr)
}
