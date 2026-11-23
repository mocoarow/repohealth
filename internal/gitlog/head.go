package gitlog

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const (
	headFields     = 2
	modeRegular    = "100644"
	modeExecutable = "100755"
	// dirtyPathsShown is how many changed paths ErrDirtyWorktree names.
	dirtyPathsShown = 5
	// statusCodeLen is the width of the XY status code; an entry is the code, a space and the path.
	statusCodeLen   = 2
	statusPrefixLen = statusCodeLen + 1
)

var (
	// ErrDirtyWorktree reports tracked changes that a measurement of HEAD would not include.
	ErrDirtyWorktree = errors.New("worktree has uncommitted changes")
	// ErrShallowRepository reports a history cut short, which makes churn and merge counts wrong.
	ErrShallowRepository = errors.New("repository is shallow")
)

// Commit identifies a commit and when its committer recorded it.
type Commit struct {
	Hash        string
	CommittedAt time.Time
}

// Head resolves HEAD in dir once, so later measurements all read the same commit.
func Head(ctx context.Context, dir string) (Commit, error) {
	out, err := run(dir, nil, exec.CommandContext(ctx, "git", "log", "-1", "--no-show-signature",
		"--format=%H"+fieldSep+"%cI"))
	if err != nil {
		return Commit{}, err
	}

	line := strings.TrimSuffix(out, "\n")

	fields := strings.Split(line, fieldSep)
	if len(fields) != headFields {
		return Commit{}, fmt.Errorf("%w: %q", ErrUnexpectedOutput, line)
	}

	committedAt, err := time.Parse(time.RFC3339, fields[1])
	if err != nil {
		return Commit{}, fmt.Errorf("%w: date of %s: %w", ErrUnexpectedOutput, fields[0], err)
	}

	return Commit{Hash: fields[0], CommittedAt: committedAt}, nil
}

// HeadFiles lists the regular files of commit rev under dir, relative to dir.
func HeadFiles(ctx context.Context, dir, rev string) ([]string, error) {
	out, err := run(dir, nil, exec.CommandContext(ctx, "git", "ls-tree", "-r", "-z", rev)) //nolint:gosec // argv entries, no shell
	if err != nil {
		return nil, err
	}

	files := []string{}

	for entry := range strings.SplitSeq(out, "\x00") {
		if entry == "" {
			continue
		}

		info, name, ok := strings.Cut(entry, "\t")
		if !ok {
			return nil, fmt.Errorf("%w: ls-tree entry %q", ErrUnexpectedOutput, entry)
		}

		mode, _, _ := strings.Cut(info, " ")
		if mode == modeRegular || mode == modeExecutable {
			files = append(files, name)
		}
	}

	return files, nil
}

// CheckClean fails with ErrDirtyWorktree when tracked files under dir differ from HEAD, except under ignore; an empty ignore excludes nothing.
func CheckClean(ctx context.Context, dir, ignore string) error {
	args := []string{"status", "--porcelain", "-z", "--untracked-files=no", "--", "."}
	// An empty ":(exclude)" pathspec excludes everything, which would hide every change.
	if ignore != "" {
		args = append(args, ":(exclude)"+ignore)
	}

	out, err := run(dir, nil, exec.CommandContext(ctx, "git", args...))
	if err != nil {
		return err
	}

	paths := changedPaths(out)
	if len(paths) == 0 {
		return nil
	}

	shown := paths[:min(len(paths), dirtyPathsShown)]
	msg := strings.Join(shown, ", ")

	if more := len(paths) - len(shown); more > 0 {
		msg += fmt.Sprintf(" (+%d more)", more)
	}

	return fmt.Errorf("%w: %s", ErrDirtyWorktree, msg)
}

// changedPaths lists the paths of `git status --porcelain -z` output; a rename or copy entry is followed by its
// source path, which is not a changed path of its own.
func changedPaths(out string) []string {
	paths := []string{}
	entries := strings.Split(out, "\x00")

	for i := 0; i < len(entries); i++ {
		entry := entries[i]
		if len(entry) < statusPrefixLen {
			continue
		}

		paths = append(paths, entry[statusPrefixLen:])

		if strings.ContainsAny(entry[:statusCodeLen], "RC") {
			i++
		}
	}

	return paths
}

// CheckComplete fails with ErrShallowRepository when the history in dir is truncated.
func CheckComplete(ctx context.Context, dir string) error {
	out, err := run(dir, nil, exec.CommandContext(ctx, "git", "rev-parse", "--is-shallow-repository"))
	if err != nil {
		return err
	}

	if strings.TrimSpace(out) == "true" {
		return ErrShallowRepository
	}

	return nil
}
