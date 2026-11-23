package coverage

import (
	"bufio"
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mocoarow/repohealth/internal/config"
)

const (
	rootLayer = "."
	daFields  = 2
)

// ErrMalformedReport reports an lcov line this package cannot read.
var ErrMalformedReport = errors.New("malformed lcov report")

// Report is one lcov file that was read.
type Report struct {
	File    string    `json:"file"`
	ModTime time.Time `json:"modTime"`
	// Skipped counts the SF records that point outside the config directory.
	Skipped int `json:"skipped"`
}

// Layer is the line coverage of one top-level directory of an area.
type Layer struct {
	Area    string `json:"area"`
	Layer   string `json:"layer"`
	Covered int    `json:"covered"`
	Lines   int    `json:"lines"`
}

// Result is the coverage of every report in the configuration.
type Result struct {
	Reports []Report `json:"reports"`
	Layers  []Layer  `json:"layers"`
}

// Measure reads the configured lcov reports and sums their lines per area and layer.
func Measure(fsys fs.FS, cfg config.Config) (Result, error) {
	reports := make([]Report, 0, len(cfg.Coverage))
	lines := map[string]map[int]bool{}

	for _, r := range cfg.Coverage {
		info, err := fs.Stat(fsys, r.File)
		if err != nil {
			return Result{}, fmt.Errorf("stat %s: %w", r.File, err)
		}

		data, err := fs.ReadFile(fsys, r.File)
		if err != nil {
			return Result{}, fmt.Errorf("read %s: %w", r.File, err)
		}

		skipped, err := parse(data, cfg.Root, r.Root, lines)
		if err != nil {
			return Result{}, fmt.Errorf("%s: %w", r.File, err)
		}

		reports = append(reports, Report{File: r.File, ModTime: info.ModTime(), Skipped: skipped})
	}

	return Result{Reports: reports, Layers: layers(cfg, lines)}, nil
}

func parse(data []byte, repoRoot, root string, lines map[string]map[int]bool) (int, error) {
	var (
		file     string
		inRecord bool
		skipped  int
	)

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		key, value, _ := strings.Cut(scanner.Text(), ":")

		switch key {
		case "SF":
			file, inRecord = sourcePath(repoRoot, root, value), true
			if file == "" {
				skipped++
			}
		case "DA":
			line, hits, err := lineHits(value)
			if err != nil || !inRecord {
				return 0, fmt.Errorf("%w: DA:%s", ErrMalformedReport, value)
			}

			if file == "" {
				continue
			}

			if lines[file] == nil {
				lines[file] = map[int]bool{}
			}

			lines[file][line] = lines[file][line] || hits > 0
		case "end_of_record":
			file, inRecord = "", false
		}
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("scan: %w", err)
	}

	return skipped, nil
}

func sourcePath(repoRoot, root, sf string) string {
	p := path.Join(root, sf)

	if filepath.IsAbs(filepath.FromSlash(sf)) {
		rel, err := filepath.Rel(repoRoot, filepath.FromSlash(sf))
		if err != nil {
			return ""
		}

		p = filepath.ToSlash(rel)
	}

	if !filepath.IsLocal(filepath.FromSlash(p)) {
		return ""
	}

	return p
}

func lineHits(value string) (int, int, error) {
	fields := strings.Split(value, ",")
	if len(fields) < daFields {
		return 0, 0, ErrMalformedReport
	}

	line, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, fmt.Errorf("line: %w", err)
	}

	hits, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, fmt.Errorf("hits: %w", err)
	}

	return line, hits, nil
}

type layerKey struct {
	area  string
	layer string
}

func layers(cfg config.Config, lines map[string]map[int]bool) []Layer {
	sums := map[layerKey]Layer{}

	for file, covered := range lines {
		area, ok := cfg.AreaOf(file)
		if !ok {
			continue
		}

		dir, _ := cfg.PathOf(file)
		key := layerKey{area: area, layer: layerOf(dir, file)}
		sum := sums[key]

		for _, c := range covered {
			sum.Lines++
			if c {
				sum.Covered++
			}
		}

		sums[key] = Layer{Area: key.area, Layer: key.layer, Covered: sum.Covered, Lines: sum.Lines}
	}

	result := make([]Layer, 0, len(sums))
	for _, l := range sums {
		result = append(result, l)
	}

	slices.SortFunc(result, func(a, b Layer) int {
		return cmp.Or(cmp.Compare(a.Area, b.Area), cmp.Compare(a.Layer, b.Layer))
	})

	return result
}

func layerOf(dir, file string) string {
	rel := file
	if dir != "." {
		rel = strings.TrimPrefix(file, dir+"/")
	}

	layer, _, found := strings.Cut(rel, "/")
	if !found {
		return rootLayer
	}

	return layer
}
