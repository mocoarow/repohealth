package complexity

import (
	"bytes"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mocoarow/repohealth/internal/config"
)

const (
	biomeCategory = "lint/complexity/noExcessiveCognitiveComplexity"
	biomeConfig   = `{"linter":{"rules":{"complexity":{"noExcessiveCognitiveComplexity":` +
		`{"level":"error","options":{"maxAllowedComplexity":1}}}}}}`
	anonymous       = "(anonymous)"
	biomeExitErrors = 1
	configFileMode  = 0o600
)

var (
	// ErrUnexpectedBiomeOutput reports Biome output this package cannot read.
	ErrUnexpectedBiomeOutput = errors.New("unexpected biome output")

	complexityMessage = regexp.MustCompile(`^Excessive complexity of (\d+) detected`)
	identifier        = regexp.MustCompile(`^[\p{L}_$][\p{L}\p{N}_$]*$`)
)

type biomeReport struct {
	Diagnostics []biomeDiagnostic `json:"diagnostics"`
}

type biomeDiagnostic struct {
	Category string        `json:"category"`
	Message  string        `json:"message"`
	Location biomeLocation `json:"location"`
}

type biomeLocation struct {
	Path  string   `json:"path"`
	Start biomePos `json:"start"`
	End   biomePos `json:"end"`
}

type biomePos struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// TypeScript measures, with Biome, the cognitive complexity of non-test scripts among files that belong to an area.
func TypeScript(ctx context.Context, cfg config.Config, files []string) ([]Function, error) {
	byDir := map[string]map[string]bool{}

	for _, file := range files {
		dir, ok := cfg.PathOf(file)
		if !ok || !isScript(file) {
			continue
		}

		if byDir[dir] == nil {
			byDir[dir] = map[string]bool{}
		}

		byDir[dir][file] = true
	}

	funcs := []Function{}

	for dir, targets := range byDir {
		found, err := lintDir(ctx, cfg.Root, dir, targets)
		if err != nil {
			return nil, err
		}

		funcs = append(funcs, found...)
	}

	slices.SortFunc(funcs, func(a, b Function) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line))
	})

	return funcs, nil
}

func isScript(file string) bool {
	switch path.Ext(file) {
	case ".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs":
	default:
		return false
	}

	base := path.Base(file)

	return !strings.HasSuffix(base, ".d.ts") && !strings.Contains(base, ".test.") && !strings.Contains(base, ".spec.")
}

func lintDir(ctx context.Context, root, dir string, targets map[string]bool) ([]Function, error) {
	out, stderr, err := runBiome(ctx, filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		return nil, fmt.Errorf("biome in %s: %w", dir, err)
	}

	diagnostics, err := parseBiome(out)
	if err != nil {
		return nil, fmt.Errorf("biome in %s: %w: %s", dir, err, stderr)
	}

	sources := os.DirFS(root)
	funcs := []Function{}

	for _, d := range diagnostics {
		file := path.Join(dir, d.Location.Path)
		if !targets[file] {
			continue
		}

		f, err := toFunction(sources, file, d)
		if err != nil {
			return nil, err
		}

		funcs = append(funcs, f)
	}

	return funcs, nil
}

func runBiome(ctx context.Context, dir string) ([]byte, string, error) {
	configDir, err := os.MkdirTemp("", "repohealth-biome-")
	if err != nil {
		return nil, "", fmt.Errorf("create biome config dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(configDir); err != nil {
			slog.Error("remove biome config dir", "dir", configDir, "error", err)
		}
	}()

	if err := os.WriteFile(filepath.Join(configDir, "biome.json"), []byte(biomeConfig), configFileMode); err != nil {
		return nil, "", fmt.Errorf("write biome config: %w", err)
	}

	cmd := exec.CommandContext(ctx, "biome", "lint", "--only=complexity/noExcessiveCognitiveComplexity",
		"--reporter=json", "--max-diagnostics=none", "--colors=off", ".")

	var stdout, stderr bytes.Buffer

	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BIOME_CONFIG_PATH="+configDir)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	errText := strings.TrimSpace(stderr.String())
	if err != nil && !reportedErrors(err) {
		return nil, "", fmt.Errorf("run: %w: %s", err, errText)
	}

	return stdout.Bytes(), errText, nil
}

// reportedErrors is true for exit code 1, which Biome uses both for lint errors and for its own failures.
func reportedErrors(err error) bool {
	var exitErr *exec.ExitError

	return errors.As(err, &exitErr) && exitErr.ExitCode() == biomeExitErrors
}

func parseBiome(out []byte) ([]biomeDiagnostic, error) {
	var report biomeReport
	if err := json.Unmarshal(out, &report); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnexpectedBiomeOutput, err)
	}

	diagnostics := []biomeDiagnostic{}

	for _, d := range report.Diagnostics {
		if d.Category == biomeCategory {
			diagnostics = append(diagnostics, d)
		}
	}

	return diagnostics, nil
}

func toFunction(sources fs.FS, file string, d biomeDiagnostic) (Function, error) {
	cognit, err := cognitOf(d.Message)
	if err != nil {
		return Function{}, err
	}

	name, err := nameAt(sources, file, d.Location)
	if err != nil {
		return Function{}, err
	}

	return Function{File: file, Line: d.Location.Start.Line, Name: name, Cyclo: CycloNotMeasured, Cognit: cognit}, nil
}

func cognitOf(message string) (int, error) {
	m := complexityMessage.FindStringSubmatch(message)
	if m == nil {
		return 0, fmt.Errorf("%w: message %q", ErrUnexpectedBiomeOutput, message)
	}

	cognit, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, fmt.Errorf("%w: message %q: %w", ErrUnexpectedBiomeOutput, message, err)
	}

	return cognit, nil
}

func nameAt(sources fs.FS, file string, loc biomeLocation) (string, error) {
	src, err := fs.ReadFile(sources, file)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", file, err)
	}

	lines := strings.Split(string(src), "\n")
	if loc.Start.Line < 1 || loc.Start.Line > len(lines) || loc.Start.Line != loc.End.Line {
		return anonymous, nil
	}

	line := []rune(lines[loc.Start.Line-1])
	if loc.Start.Column < 1 || loc.End.Column > len(line)+1 || loc.Start.Column >= loc.End.Column {
		return anonymous, nil
	}

	name := string(line[loc.Start.Column-1 : loc.End.Column-1])
	if name == "function" || !identifier.MatchString(name) {
		return anonymous, nil
	}

	return name, nil
}
