package gitlog

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

const (
	fieldSep     = "\x1f"
	headerFields = 3
)

// ErrUnexpectedOutput reports git output this package cannot read.
var ErrUnexpectedOutput = errors.New("unexpected git output")

// Merge is a merge commit on the first-parent history of a revision.
type Merge struct {
	Hash        string
	CommittedAt time.Time
	// Files changed against the first parent, relative to the directory given to Merges.
	Files []string
}

type header struct {
	hash        string
	firstParent string
	committedAt time.Time
}

// Merges lists the merge commits on the first-parent history of rev in dir.
func Merges(ctx context.Context, dir, rev string) ([]Merge, error) {
	out, err := run(dir, nil, exec.CommandContext(ctx, "git", "log", "--first-parent", "--merges", //nolint:gosec // argv entries, no shell
		"--no-show-signature", "--format=%H"+fieldSep+"%P"+fieldSep+"%cI", rev))
	if err != nil {
		return nil, err
	}

	merges := []Merge{}

	for line := range strings.Lines(out) {
		h, err := parseHeader(strings.TrimSuffix(line, "\n"))
		if err != nil {
			return nil, err
		}

		files, err := changedFiles(ctx, dir, h)
		if err != nil {
			return nil, fmt.Errorf("files of merge %s: %w", h.hash, err)
		}

		merges = append(merges, Merge{Hash: h.hash, CommittedAt: h.committedAt, Files: files})
	}

	return merges, nil
}

func parseHeader(line string) (header, error) {
	fields := strings.Split(line, fieldSep)
	if len(fields) != headerFields {
		return header{}, fmt.Errorf("%w: %q", ErrUnexpectedOutput, line)
	}

	parents := strings.Fields(fields[1])
	if len(parents) == 0 {
		return header{}, fmt.Errorf("%w: no parent in %q", ErrUnexpectedOutput, line)
	}

	committedAt, err := time.Parse(time.RFC3339, fields[2])
	if err != nil {
		return header{}, fmt.Errorf("%w: date of %s: %w", ErrUnexpectedOutput, fields[0], err)
	}

	return header{hash: fields[0], firstParent: parents[0], committedAt: committedAt}, nil
}

func changedFiles(ctx context.Context, dir string, h header) ([]string, error) {
	out, err := run(dir, strings.NewReader(h.hash+" "+h.firstParent+"\n"), exec.CommandContext(ctx, "git",
		"diff-tree", "--stdin", "--always", "-z", "-r", "--name-only", "--no-renames", "--relative"))
	if err != nil {
		return nil, err
	}

	tokens := strings.Split(strings.TrimSuffix(out, "\x00"), "\x00")
	if tokens[0] != h.hash {
		return nil, fmt.Errorf("%w: diff-tree of %s starts with %q", ErrUnexpectedOutput, h.hash, tokens[0])
	}

	return tokens[1:], nil
}

func run(dir string, stdin io.Reader, cmd *exec.Cmd) (string, error) {
	var stderr bytes.Buffer

	cmd.Dir = dir
	cmd.Stdin = stdin
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s in %s: %w: %s", cmd.Args[1], dir, err, strings.TrimSpace(stderr.String()))
	}

	return string(out), nil
}
