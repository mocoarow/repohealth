package snapshot

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mocoarow/repohealth/internal/complexity"
	"github.com/mocoarow/repohealth/internal/config"
	"github.com/mocoarow/repohealth/internal/coverage"
	"github.com/mocoarow/repohealth/internal/crossarea"
	"github.com/mocoarow/repohealth/internal/gitlog"
	"github.com/mocoarow/repohealth/internal/godeps"
	"github.com/mocoarow/repohealth/internal/size"
	"github.com/mocoarow/repohealth/internal/source"
)

const (
	// SchemaVersion is the version of the Summary and Detail JSON layout.
	SchemaVersion = 1
	// CognitLimit is the cognitive complexity a function may reach before it counts as over the limit.
	CognitLimit = 20
	// DetailLimit is how many functions per language, hotspots and largest files a Detail keeps.
	DetailLimit = 30
	// RecentDays is how far back from the HEAD commit recent churn counts.
	RecentDays = 90
)

// ErrBiomeVersionMismatch reports that Biome differs between the area directories it lints, so no single version applies.
var ErrBiomeVersionMismatch = errors.New("biome version differs between areas")

// Header identifies a snapshot: when it was taken, of which commit, and with which tools.
type Header struct {
	SchemaVersion int       `json:"schemaVersion"`
	TakenAt       time.Time `json:"takenAt"`
	Rev           string    `json:"rev"`
	CommittedAt   time.Time `json:"committedAt"`
	Tools         Tools     `json:"tools"`
}

// Tools are the versions of the programs that measured a snapshot; Biome is empty when no area holds a non-test script.
type Tools struct {
	Repohealth string `json:"repohealth"`
	Biome      string `json:"biome"`
}

// Input is what Take needs to measure a repository.
type Input struct {
	Config  config.Config
	TakenAt time.Time
	// Version is the repohealth version recorded in the header.
	Version string
	// OutputDir is the directory, relative to the config root, whose changes do not make the worktree dirty.
	OutputDir string
}

// Measurements are the raw results that Summarize and Detailed shape.
type Measurements struct {
	CrossArea crossarea.Result
	Go        []complexity.Function
	TS        []complexity.Function
	Churn     map[string]gitlog.FileChurn
	Graph     godeps.Graph
	Coverage  coverage.Result
	Sizes     []size.File
}

// Take measures the HEAD commit of the repository holding in.Config and returns its summary and detail.
func Take(ctx context.Context, in Input) (Summary, Detail, error) {
	cfg := in.Config

	if err := gitlog.CheckComplete(ctx, cfg.Root); err != nil {
		return Summary{}, Detail{}, fmt.Errorf("check history: %w", err)
	}

	if err := gitlog.CheckClean(ctx, cfg.Root, in.OutputDir); err != nil {
		return Summary{}, Detail{}, fmt.Errorf("check worktree: %w", err)
	}

	head, err := gitlog.Head(ctx, cfg.Root)
	if err != nil {
		return Summary{}, Detail{}, fmt.Errorf("resolve HEAD: %w", err)
	}

	files, err := gitlog.HeadFiles(ctx, cfg.Root, head.Hash)
	if err != nil {
		return Summary{}, Detail{}, fmt.Errorf("list files of %s: %w", head.Hash, err)
	}

	biome, err := biomeVersion(ctx, cfg, files)
	if err != nil {
		return Summary{}, Detail{}, err
	}

	m, err := measure(ctx, cfg, head, files)
	if err != nil {
		return Summary{}, Detail{}, fmt.Errorf("measure %s: %w", head.Hash, err)
	}

	h := Header{
		SchemaVersion: SchemaVersion,
		TakenAt:       in.TakenAt,
		Rev:           head.Hash,
		CommittedAt:   head.CommittedAt,
		Tools:         Tools{Repohealth: in.Version, Biome: biome},
	}

	return Summarize(cfg, h, m), Detailed(cfg, h, m), nil
}

func measure(ctx context.Context, cfg config.Config, head gitlog.Commit, files []string) (Measurements, error) {
	merges, err := gitlog.Merges(ctx, cfg.Root, head.Hash)
	if err != nil {
		return Measurements{}, fmt.Errorf("list merges: %w", err)
	}

	churn, err := gitlog.Churn(ctx, cfg.Root, head.Hash, head.CommittedAt.AddDate(0, 0, -RecentDays))
	if err != nil {
		return Measurements{}, fmt.Errorf("count churn: %w", err)
	}

	m, err := measureFiles(ctx, cfg, files)
	if err != nil {
		return Measurements{}, err
	}

	return Measurements{
		CrossArea: crossarea.Measure(cfg, changesOf(merges)),
		Go:        m.goFuncs,
		TS:        m.tsFuncs,
		Churn:     churn,
		Graph:     m.graph,
		Coverage:  m.coverage,
		Sizes:     m.sizes,
	}, nil
}

type fileMeasurements struct {
	goFuncs  []complexity.Function
	tsFuncs  []complexity.Function
	graph    godeps.Graph
	coverage coverage.Result
	sizes    []size.File
}

func measureFiles(ctx context.Context, cfg config.Config, files []string) (fileMeasurements, error) {
	fsys := os.DirFS(cfg.Root)

	goFuncs, err := complexity.Go(fsys, cfg, files)
	if err != nil {
		return fileMeasurements{}, fmt.Errorf("measure go complexity: %w", err)
	}

	tsFuncs, err := complexity.TypeScript(ctx, cfg, files)
	if err != nil {
		return fileMeasurements{}, fmt.Errorf("measure typescript complexity: %w", err)
	}

	graph, err := godeps.Load(fsys, cfg, files)
	if err != nil {
		return fileMeasurements{}, fmt.Errorf("load go imports: %w", err)
	}

	cov, err := coverage.Measure(fsys, cfg)
	if err != nil {
		return fileMeasurements{}, fmt.Errorf("measure coverage: %w", err)
	}

	sizes, err := size.Measure(fsys, cfg, files)
	if err != nil {
		return fileMeasurements{}, fmt.Errorf("measure sizes: %w", err)
	}

	return fileMeasurements{goFuncs: goFuncs, tsFuncs: tsFuncs, graph: graph, coverage: cov, sizes: sizes}, nil
}

func changesOf(merges []gitlog.Merge) []crossarea.Change {
	changes := make([]crossarea.Change, 0, len(merges))
	for _, m := range merges {
		changes = append(changes, crossarea.Change{MergedAt: m.CommittedAt, Files: m.Files})
	}

	return changes
}

// biomeVersion asks Biome only in area directories that hold scripts to lint, so other repositories do not need it
// installed. Each directory is asked because the version manager resolves Biome per directory, as lint runs it.
func biomeVersion(ctx context.Context, cfg config.Config, files []string) (string, error) {
	dirs := map[string]bool{}

	for _, file := range files {
		if !source.ScriptProduction(cfg, file) {
			continue
		}

		area, _ := cfg.PathOf(file)
		dirs[filepath.Join(cfg.Root, filepath.FromSlash(area))] = true
	}

	versions := make(map[string]string, len(dirs))

	for _, dir := range slices.Sorted(maps.Keys(dirs)) {
		v, err := complexity.BiomeVersion(ctx, dir)
		if err != nil {
			return "", fmt.Errorf("read biome version in %s: %w", dir, err)
		}

		versions[dir] = v
	}

	return oneVersion(versions)
}

// oneVersion returns the version shared by every directory, or "" when there are none.
func oneVersion(versions map[string]string) (string, error) {
	dirsOf := map[string][]string{}

	for dir, v := range versions {
		dirsOf[v] = append(dirsOf[v], dir)
	}

	switch len(dirsOf) {
	case 0:
		return "", nil
	case 1:
		for v := range dirsOf {
			return v, nil
		}
	}

	found := make([]string, 0, len(dirsOf))

	for _, v := range slices.Sorted(maps.Keys(dirsOf)) {
		found = append(found, fmt.Sprintf("%s (%s)", v, strings.Join(slices.Sorted(slices.Values(dirsOf[v])), ", ")))
	}

	return "", fmt.Errorf("%w: %s", ErrBiomeVersionMismatch, strings.Join(found, "; "))
}

// nonNil keeps empty lists as [] in JSON.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}

	return s
}
