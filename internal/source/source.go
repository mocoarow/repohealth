package source

import (
	"path"
	"strings"

	"github.com/mocoarow/repohealth/internal/config"
)

// Lang is the language of a source file.
type Lang int

const (
	_ Lang = iota
	// Go is Go source.
	Go
	// Script is TypeScript or JavaScript source.
	Script
)

// Kind is what a measured source file is.
type Kind struct {
	Lang Lang
	Test bool
}

// Classify reports what file is, and false when no measurement reads it.
func Classify(cfg config.Config, file string) (Kind, bool) {
	if _, ok := cfg.AreaOf(file); !ok {
		return Kind{Lang: 0, Test: false}, false
	}

	switch {
	case strings.HasSuffix(file, ".go") && !ignoredByGo(path.Dir(file)):
		return Kind{Lang: Go, Test: strings.HasSuffix(file, "_test.go")}, true
	case isScript(file):
		base := path.Base(file)

		return Kind{Lang: Script, Test: strings.Contains(base, ".test.") || strings.Contains(base, ".spec.")}, true
	default:
		return Kind{Lang: 0, Test: false}, false
	}
}

// GoProduction reports whether file is a non-test Go source that measurements read.
func GoProduction(cfg config.Config, file string) bool {
	k, ok := Classify(cfg, file)

	return ok && k.Lang == Go && !k.Test
}

func isScript(file string) bool {
	switch path.Ext(file) {
	case ".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs":
		return !strings.HasSuffix(file, ".d.ts")
	default:
		return false
	}
}

func ignoredByGo(dir string) bool {
	for elem := range strings.SplitSeq(dir, "/") {
		if elem == "testdata" || (elem != "." && (strings.HasPrefix(elem, ".") || strings.HasPrefix(elem, "_"))) {
			return true
		}
	}

	return false
}
