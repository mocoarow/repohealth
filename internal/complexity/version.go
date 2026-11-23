package complexity

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var biomeVersionLine = regexp.MustCompile(`(?m)^Version:\s*(\S+)\s*$`)

// BiomeVersion returns the version of the Biome that runs in dir.
func BiomeVersion(ctx context.Context, dir string) (string, error) {
	cmd := exec.CommandContext(ctx, "biome", "--version")
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("run biome --version: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return parseBiomeVersion(stdout.String())
}

func parseBiomeVersion(out string) (string, error) {
	m := biomeVersionLine.FindStringSubmatch(strings.ReplaceAll(out, "\r\n", "\n"))
	if m == nil {
		return "", fmt.Errorf("%w: version %q", ErrUnexpectedBiomeOutput, out)
	}

	return m[1], nil
}
