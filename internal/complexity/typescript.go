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
	"github.com/mocoarow/repohealth/internal/source"
)

const (
	internalCategory = "internalError"
	parseCategory    = "parse"
	biomeCategory    = "lint/complexity/noExcessiveCognitiveComplexity"
	// files.maxSize is in bytes; Biome skips larger files silently, so the limit is raised far beyond real sources.
	biomeConfig = `{"files":{"maxSize":4294967296},"linter":{"rules":{"complexity":{"noExcessiveCognitiveComplexity":` +
		`{"level":"error","options":{"maxAllowedComplexity":1}}}}}}`
	biomeBatch      = 200
	anonymous       = "(anonymous)"
	biomeExitErrors = 1
	configFileMode  = 0o600
)

var (
	// ErrUnexpectedBiomeOutput reports Biome output this package cannot read.
	ErrUnexpectedBiomeOutput = errors.New("unexpected biome output")

	// ErrBiomeInternal reports a Biome diagnostic about its own failure, such as an unreadable file.
	ErrBiomeInternal = errors.New("biome internal error")

	// ErrBiomeParse reports a file Biome could not parse; Biome skips the broken code, so its functions would go missing.
	ErrBiomeParse = errors.New("biome parse error")

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
	byDir := map[string][]string{}

	for _, file := range files {
		if !source.ScriptProduction(cfg, file) {
			continue
		}

		dir, _ := cfg.PathOf(file)

		byDir[dir] = append(byDir[dir], file)
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

func lintDir(ctx context.Context, root, dir string, files []string) ([]Function, error) {
	configDir, err := os.MkdirTemp("", "repohealth-biome-")
	if err != nil {
		return nil, fmt.Errorf("create biome config dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(configDir); err != nil {
			slog.Error("remove biome config dir", "dir", configDir, "error", err)
		}
	}()

	if err := os.WriteFile(filepath.Join(configDir, "biome.json"), []byte(biomeConfig), configFileMode); err != nil {
		return nil, fmt.Errorf("write biome config: %w", err)
	}

	sources := os.DirFS(root)
	funcs := []Function{}

	for batch := range slices.Chunk(files, biomeBatch) {
		found, err := lintBatch(ctx, batchRun{
			sources:   sources,
			workDir:   filepath.Join(root, filepath.FromSlash(dir)),
			configDir: configDir,
			area:      dir,
		}, batch)
		if err != nil {
			return nil, err
		}

		funcs = append(funcs, found...)
	}

	return funcs, nil
}

// batchRun is what one Biome invocation needs besides its files: area is the repository-relative area path,
// workDir the same directory on disk.
type batchRun struct {
	sources   fs.FS
	workDir   string
	configDir string
	area      string
}

func relativeTo(area, file string) string {
	if area == "." {
		return file
	}

	return strings.TrimPrefix(file, area+"/")
}

func lintBatch(ctx context.Context, run batchRun, files []string) ([]Function, error) {
	dir := run.area
	rel := make([]string, 0, len(files))

	for _, file := range files {
		rel = append(rel, relativeTo(dir, file))
	}

	out, stderr, err := runBiome(ctx, run.workDir, run.configDir, rel)
	if err != nil {
		return nil, fmt.Errorf("biome in %s: %w", dir, err)
	}

	diagnostics, err := parseBiome(out)
	if err != nil {
		return nil, fmt.Errorf("biome in %s: %w: %s", dir, err, stderr)
	}

	return functionsOf(run.sources, dir, files, diagnostics)
}

// functionsOf converts diagnostics to functions; every diagnostic must belong to a requested file.
func functionsOf(sources fs.FS, dir string, files []string, diagnostics []biomeDiagnostic) ([]Function, error) {
	targets := map[string]bool{}
	for _, file := range files {
		targets[file] = true
	}

	funcs := []Function{}

	for _, d := range diagnostics {
		file := path.Join(dir, d.Location.Path)
		if !targets[file] {
			return nil, fmt.Errorf("%w: diagnostic for %s, which was not requested", ErrUnexpectedBiomeOutput, file)
		}

		f, err := toFunction(sources, file, d)
		if err != nil {
			return nil, err
		}

		funcs = append(funcs, f)
	}

	return funcs, nil
}

func runBiome(ctx context.Context, dir, configDir string, files []string) ([]byte, string, error) {
	args := append([]string{
		"lint", "--only=complexity/noExcessiveCognitiveComplexity",
		"--reporter=json", "--max-diagnostics=none", "--colors=off", "--",
	}, files...)
	cmd := exec.CommandContext(ctx, "biome", args...) //nolint:gosec // arguments follow "--", so file names cannot act as options

	var stdout, stderr bytes.Buffer

	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "BIOME_CONFIG_PATH="+configDir)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

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
		if strings.HasPrefix(d.Category, internalCategory) {
			return nil, fmt.Errorf("%w: %s: %s: %s", ErrBiomeInternal, d.Category, d.Location.Path, d.Message)
		}

		if d.Category == parseCategory {
			return nil, fmt.Errorf("%w: %s: %s", ErrBiomeParse, d.Location.Path, d.Message)
		}

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
