package cli

import (
	"context"
	"io"

	"github.com/mocoarow/repohealth/internal/report"
)

func runMarkdown(_ context.Context, args []string, stdout, stderr io.Writer) error {
	return historyOutput{
		name:       "markdown",
		outHelp:    "file to write the Markdown report to (required)",
		write:      report.WriteMarkdown,
		donePrefix: "wrote ",
	}.run(args, stdout, stderr)
}
