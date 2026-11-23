package gitlog

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	churnMarker = "\x1e"
	// A commit header is followed by a newline that belongs to git's -z output, not to the first file name.
	headerNewline = "\n"
)

// FileChurn counts the commits that changed a file: all of them, and those committed since a cutoff.
type FileChurn struct {
	Total  int
	Recent int
}

// Churn counts, per file under dir, the non-merge commits reachable from rev that changed it.
func Churn(ctx context.Context, dir, rev string, since time.Time) (map[string]FileChurn, error) {
	// --since would stop at the first old commit, but committer dates need not increase along the history.
	out, err := run(dir, nil, exec.CommandContext(ctx, "git", "log", "--root", //nolint:gosec // argv entries, no shell
		"--no-merges", "--no-show-signature",
		"--format="+churnMarker+"%H"+fieldSep+"%cI", "--name-only", "-z", "--no-renames", "--relative", rev))
	if err != nil {
		return nil, err
	}

	churn := map[string]FileChurn{}
	recent := false
	afterHeader := false

	for token := range strings.SplitSeq(out, "\x00") {
		if afterHeader {
			token = strings.TrimPrefix(token, headerNewline)
			afterHeader = false
		}

		committedAt, ok, err := parseChurnHeader(token)
		if err != nil {
			return nil, err
		}

		if ok {
			recent = !committedAt.Before(since)
			afterHeader = true

			continue
		}

		if token == "" {
			continue
		}

		c := churn[token]
		c.Total++

		if recent {
			c.Recent++
		}

		churn[token] = c
	}

	return churn, nil
}

func parseChurnHeader(token string) (time.Time, bool, error) {
	rest, ok := strings.CutPrefix(token, churnMarker)
	if !ok {
		return time.Time{}, false, nil
	}

	hash, date, ok := strings.Cut(rest, fieldSep)
	if !ok || !isHash(hash) {
		return time.Time{}, false, fmt.Errorf("%w: churn header %q", ErrUnexpectedOutput, token)
	}

	committedAt, err := time.Parse(time.RFC3339, date)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("%w: date of %s: %w", ErrUnexpectedOutput, hash, err)
	}

	return committedAt, true, nil
}

func isHash(s string) bool {
	const sha1Len, sha256Len = 40, 64

	if len(s) != sha1Len && len(s) != sha256Len {
		return false
	}

	for _, r := range s {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return false
		}
	}

	return true
}
