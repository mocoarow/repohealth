package godeps

import (
	"cmp"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"

	"github.com/mocoarow/repohealth/internal/config"
	"github.com/mocoarow/repohealth/internal/gosrc"
)

// ErrNoModule reports a Go package without a usable go.mod above it.
var ErrNoModule = errors.New("no module path for package")

// Package is one Go package of the repository and the repository packages it imports.
type Package struct {
	Dir     string
	Path    string
	Area    string
	Imports []string
}

// Graph is the import graph between the repository's own packages.
type Graph struct {
	Packages []Package
}

// AreaImports counts the package imports from one area into another.
type AreaImports struct {
	From    string
	To      string
	Imports int
}

// Stability is the instability of one package.
type Stability struct {
	Dir         string
	Afferent    int
	Efferent    int
	Instability float64
}

type scanned struct {
	area    string
	imports map[string]bool
}

// Load builds the graph from the files gosrc.Counted accepts.
func Load(fsys fs.FS, cfg config.Config, files []string) (Graph, error) {
	byDir, err := scan(fsys, cfg, files)
	if err != nil {
		return Graph{}, err
	}

	modules := moduleFinder{fsys: fsys, paths: map[string]string{}}
	importPaths := map[string]string{}

	for dir := range byDir {
		p, err := modules.importPath(dir)
		if err != nil {
			return Graph{}, err
		}

		importPaths[dir] = p
	}

	known := map[string]bool{}
	for _, p := range importPaths {
		known[p] = true
	}

	packages := make([]Package, 0, len(byDir))

	for dir, s := range byDir {
		var imports []string

		for imp := range s.imports {
			if known[imp] {
				imports = append(imports, imp)
			}
		}

		slices.Sort(imports)
		packages = append(packages, Package{Dir: dir, Path: importPaths[dir], Area: s.area, Imports: imports})
	}

	slices.SortFunc(packages, func(a, b Package) int { return cmp.Compare(a.Dir, b.Dir) })

	return Graph{Packages: packages}, nil
}

func scan(fsys fs.FS, cfg config.Config, files []string) (map[string]scanned, error) {
	byDir := map[string]scanned{}

	for _, file := range files {
		if !gosrc.Counted(cfg, file) {
			continue
		}

		area, _ := cfg.AreaOf(file)

		imports, err := importsOf(fsys, file)
		if err != nil {
			return nil, err
		}

		dir := path.Dir(file)

		s, seen := byDir[dir]
		if !seen {
			s = scanned{area: area, imports: map[string]bool{}}
		}

		for _, imp := range imports {
			s.imports[imp] = true
		}

		byDir[dir] = s
	}

	return byDir, nil
}

func importsOf(fsys fs.FS, file string) ([]string, error) {
	src, err := fs.ReadFile(fsys, file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}

	f, err := parser.ParseFile(token.NewFileSet(), file, src, parser.ImportsOnly)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}

	imports := make([]string, 0, len(f.Imports))

	for _, spec := range f.Imports {
		p, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, fmt.Errorf("import path in %s: %w", file, err)
		}

		imports = append(imports, p)
	}

	return imports, nil
}

type moduleFinder struct {
	fsys  fs.FS
	paths map[string]string
}

func (m moduleFinder) importPath(dir string) (string, error) {
	for modDir := dir; ; modDir = path.Dir(modDir) {
		module, err := m.modulePath(modDir)
		if err != nil {
			return "", err
		}

		if module != "" {
			return path.Join(module, relative(modDir, dir)), nil
		}

		if modDir == "." {
			return "", fmt.Errorf("%w: %s", ErrNoModule, dir)
		}
	}
}

func relative(modDir, dir string) string {
	switch modDir {
	case dir:
		return ""
	default:
		return strings.TrimPrefix(dir, modDir+"/")
	}
}

func (m moduleFinder) modulePath(dir string) (string, error) {
	if p, ok := m.paths[dir]; ok {
		return p, nil
	}

	data, err := fs.ReadFile(m.fsys, path.Join(dir, "go.mod"))

	switch {
	case errors.Is(err, fs.ErrNotExist):
		m.paths[dir] = ""
	case err != nil:
		return "", fmt.Errorf("read %s/go.mod: %w", dir, err)
	default:
		module := modfile.ModulePath(data)
		if module == "" {
			return "", fmt.Errorf("%w: %s has no module line", ErrNoModule, path.Join(dir, "go.mod"))
		}

		m.paths[dir] = module
	}

	return m.paths[dir], nil
}

// AreaImports counts the package imports between different areas.
func (g Graph) AreaImports() []AreaImports {
	areaOf := map[string]string{}
	for _, p := range g.Packages {
		areaOf[p.Path] = p.Area
	}

	counts := map[[2]string]int{}

	for _, p := range g.Packages {
		for _, imp := range p.Imports {
			if to := areaOf[imp]; to != p.Area {
				counts[[2]string{p.Area, to}]++
			}
		}
	}

	edges := make([]AreaImports, 0, len(counts))
	for k, n := range counts {
		edges = append(edges, AreaImports{From: k[0], To: k[1], Imports: n})
	}

	slices.SortFunc(edges, func(a, b AreaImports) int {
		return cmp.Or(cmp.Compare(b.Imports, a.Imports), cmp.Compare(a.From, b.From), cmp.Compare(a.To, b.To))
	})

	return edges
}

// Stability returns the instability of every package whose directory cfg marks as stable.
func (g Graph) Stability(cfg config.Config) []Stability {
	afferent := map[string]int{}

	for _, p := range g.Packages {
		for _, imp := range p.Imports {
			afferent[imp]++
		}
	}

	stabilities := []Stability{}

	for _, p := range g.Packages {
		if !cfg.Stable(p.Dir) {
			continue
		}

		ca, ce := afferent[p.Path], len(p.Imports)

		instability := 0.0
		if ca+ce > 0 {
			instability = float64(ce) / float64(ca+ce)
		}

		stabilities = append(stabilities, Stability{Dir: p.Dir, Afferent: ca, Efferent: ce, Instability: instability})
	}

	return stabilities
}
