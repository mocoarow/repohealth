package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/go-playground/validator/v10"
	"go.yaml.in/yaml/v3"
)

// FileName is the default name of the configuration file.
const FileName = "repohealth.yml"

var (
	// ErrInvalidConfig wraps every problem found while parsing or validating a configuration.
	ErrInvalidConfig = errors.New("invalid config")
	// ErrParse reports a file that cannot be read as a single repohealth.yml document.
	ErrParse = errors.New("parse config")
	// ErrInvalidField reports a missing or out-of-range field.
	ErrInvalidField = errors.New("invalid field")
	// ErrInvalidAreaPath reports an area path that is absolute, leaves the repository, goes through a symbolic link, or cannot be inspected.
	ErrInvalidAreaPath = errors.New("invalid area path")
	// ErrAreaPathNotFound reports an area path that is not a directory in the repository.
	ErrAreaPathNotFound = errors.New("area path not found")
	// ErrAreaOverlap reports two area paths where one contains the other.
	ErrAreaOverlap = errors.New("area paths overlap")
	// ErrInvalidPattern reports an exclude pattern that is not a valid glob or not a clean slash-separated relative path.
	ErrInvalidPattern = errors.New("invalid exclude pattern")
	// ErrUnknownGroupArea reports a group member that is not the name of an area.
	ErrUnknownGroupArea = errors.New("group names an unknown area")
)

// Config is the content of repohealth.yml.
type Config struct {
	// Root is the absolute directory holding the configuration file; area paths are relative to it.
	Root    string   `yaml:"-"`
	Version int      `validate:"eq=1"                   yaml:"version"`
	Areas   []Area   `validate:"min=1,unique=Name,dive" yaml:"areas"`
	Groups  []Group  `validate:"unique=Name,dive"       yaml:"groups"`
	Exclude []string `validate:"dive,required"          yaml:"exclude"`
}

// Area is a named group of directories whose changes count as one unit.
type Area struct {
	Name string `validate:"required" yaml:"name"`
	// Paths are slash-separated and cleaned, e.g. "./shared/ui/" is returned as "shared/ui".
	Paths []string `validate:"min=1,dive,required" yaml:"paths"`
}

// Group is a named set of areas, such as the backend services, measured on its own.
type Group struct {
	Name  string   `validate:"required"                   yaml:"name"`
	Areas []string `validate:"min=2,unique,dive,required" yaml:"areas"`
}

type ownedPath struct {
	path string
	area string
}

// Load reads and validates the configuration file at path.
func Load(path string) (Config, error) {
	dir, err := filepath.Abs(filepath.Dir(path))
	if err != nil {
		return Config{}, fmt.Errorf("load %s: resolve directory: %w", path, err)
	}

	root, err := os.OpenRoot(dir)
	if err != nil {
		return Config{}, fmt.Errorf("load %s: open %s: %w", path, dir, err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			slog.Error("close config root", "dir", dir, "error", err)
		}
	}()

	cfg, err := decode(root, filepath.Base(path))
	if err != nil {
		return Config{}, fmt.Errorf("load %s: %w", path, err)
	}

	areas, err := validate(root, cfg)
	if err != nil {
		return Config{}, fmt.Errorf("load %s: %w", path, err)
	}

	return Config{Root: dir, Version: cfg.Version, Areas: areas, Groups: cfg.Groups, Exclude: cfg.Exclude}, nil
}

func decode(root *os.Root, name string) (Config, error) {
	data, err := root.ReadFile(name)
	if err != nil {
		return Config{}, fmt.Errorf("read: %w", err)
	}

	var cfg Config

	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("%w: %w: %w", ErrInvalidConfig, ErrParse, err)
	}

	if err := ensureNoMoreDocuments(dec); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// ensureNoMoreDocuments accepts empty documents such as a trailing "---" and rejects any document with content.
func ensureNoMoreDocuments(dec *yaml.Decoder) error {
	for {
		var extra any

		err := dec.Decode(&extra)
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("%w: %w: %w", ErrInvalidConfig, ErrParse, err)
		}

		if extra != nil {
			return fmt.Errorf("%w: %w: more than one YAML document", ErrInvalidConfig, ErrParse)
		}
	}
}

// validate checks cfg and returns its areas with cleaned paths.
func validate(root *os.Root, cfg Config) ([]Area, error) {
	v := validator.New()
	v.RegisterTagNameFunc(yamlName)

	if err := v.Struct(cfg); err != nil {
		return nil, fmt.Errorf("%w: %w: %w", ErrInvalidConfig, ErrInvalidField, err)
	}

	areas, errs := cleanAreas(root, cfg.Areas)

	errs = append(errs, validateGroups(cfg.Groups, cfg.Areas)...)
	errs = append(errs, validateExclude(cfg.Exclude)...)
	if len(errs) > 0 {
		return nil, fmt.Errorf("%w: %w", ErrInvalidConfig, errors.Join(errs...))
	}

	return areas, nil
}

func yamlName(field reflect.StructField) string {
	name, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")

	return name
}

func cleanAreas(root *os.Root, areas []Area) ([]Area, []error) {
	var (
		errs   []error
		owners []ownedPath
	)

	cleaned := make([]Area, 0, len(areas))

	for _, area := range areas {
		paths := make([]string, 0, len(area.Paths))

		for _, p := range area.Paths {
			clean, err := checkAreaPath(root, p)
			if err != nil {
				errs = append(errs, fmt.Errorf("area %s: %w", area.Name, err))

				continue
			}

			if other, ok := findOverlap(owners, clean); ok {
				errs = append(errs, fmt.Errorf("%w: %s (area %s) and %s (area %s)", ErrAreaOverlap, clean, area.Name, other.path, other.area))

				continue
			}

			owners = append(owners, ownedPath{path: clean, area: area.Name})
			paths = append(paths, clean)
		}

		cleaned = append(cleaned, Area{Name: area.Name, Paths: paths})
	}

	return cleaned, errs
}

func checkAreaPath(root *os.Root, p string) (string, error) {
	if !filepath.IsLocal(filepath.FromSlash(p)) {
		return "", fmt.Errorf("%w: %s", ErrInvalidAreaPath, p)
	}

	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(p)))
	if clean == "." {
		return clean, nil
	}

	parts := strings.Split(clean, "/")
	for i := range parts {
		if err := checkDirectory(root, filepath.Join(parts[:i+1]...)); err != nil {
			return "", fmt.Errorf("%s: %w", p, err)
		}
	}

	return clean, nil
}

// checkDirectory rejects symbolic links because git records a link as one file, so nothing under it would ever be counted.
func checkDirectory(root *os.Root, name string) error {
	info, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: %s", ErrAreaPathNotFound, name)
	}

	if err != nil {
		return fmt.Errorf("%w: %s: %w", ErrInvalidAreaPath, name, err)
	}

	if info.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("%w: %s is a symbolic link", ErrInvalidAreaPath, name)
	}

	if !info.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", ErrAreaPathNotFound, name)
	}

	return nil
}

func findOverlap(owners []ownedPath, p string) (ownedPath, bool) {
	for _, owner := range owners {
		if contains(owner.path, p) || contains(p, owner.path) {
			return owner, true
		}
	}

	return ownedPath{path: "", area: ""}, false
}

func contains(dir, p string) bool {
	return dir == "." || dir == p || strings.HasPrefix(p, dir+"/")
}

func validateGroups(groups []Group, areas []Area) []error {
	known := make(map[string]bool, len(areas))
	for _, a := range areas {
		known[a.Name] = true
	}

	var errs []error

	for _, g := range groups {
		for _, name := range g.Areas {
			if !known[name] {
				errs = append(errs, fmt.Errorf("%w: group %s: %s", ErrUnknownGroupArea, g.Name, name))
			}
		}
	}

	return errs
}

func validateExclude(patterns []string) []error {
	var errs []error

	for _, pattern := range patterns {
		if !fs.ValidPath(pattern) || !doublestar.ValidatePattern(pattern) {
			errs = append(errs, fmt.Errorf("%w: %s", ErrInvalidPattern, pattern))
		}
	}

	return errs
}
