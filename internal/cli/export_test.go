package cli

import (
	"context"
	"io"
	"time"
)

// RunAt runs args as Run does, with the clock fixed to now and the version fixed to version.
func RunAt(ctx context.Context, now time.Time, version string, args []string, stdout, stderr io.Writer) error {
	r := runner{now: func() time.Time { return now }, version: version}

	return r.run(ctx, args, stdout, stderr)
}
