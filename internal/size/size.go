package size

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"

	"github.com/mocoarow/repohealth/internal/config"
	"github.com/mocoarow/repohealth/internal/source"
)

// File is the size of one measured source file.
type File struct {
	Path  string
	Area  string
	Lang  source.Lang
	Test  bool
	Lines int
}

// Measure counts the lines of the files source.Classify accepts, in the order of files.
func Measure(fsys fs.FS, cfg config.Config, files []string) ([]File, error) {
	sizes := []File{}

	for _, file := range files {
		kind, ok := source.Classify(cfg, file)
		if !ok {
			continue
		}

		data, err := fs.ReadFile(fsys, file)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file, err)
		}

		if kind.Lang == source.Go {
			generated, err := isGenerated(file, data)
			if err != nil {
				return nil, err
			}

			if generated {
				continue
			}
		}

		area, _ := cfg.AreaOf(file)
		sizes = append(sizes, File{Path: file, Area: area, Lang: kind.Lang, Test: kind.Test, Lines: bytes.Count(data, []byte("\n"))})
	}

	return sizes, nil
}

func isGenerated(file string, src []byte) (bool, error) {
	f, err := parser.ParseFile(token.NewFileSet(), file, src, parser.PackageClauseOnly|parser.ParseComments)
	if err != nil {
		return false, fmt.Errorf("parse %s: %w", file, err)
	}

	return ast.IsGenerated(f), nil
}
