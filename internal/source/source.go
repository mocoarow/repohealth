package source

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/mocoarow/repohealth/internal/config"
)

// ErrUnknownLang reports a language that has no short name, or a short name that is no language.
var ErrUnknownLang = errors.New("unknown language")

const (
	goName     = "go"
	scriptName = "ts"
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

// MarshalText returns the short name used in JSON.
func (l Lang) MarshalText() ([]byte, error) {
	switch l {
	case Go:
		return []byte(goName), nil
	case Script:
		return []byte(scriptName), nil
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnknownLang, int(l))
	}
}

// UnmarshalText reads the short name MarshalText returns.
func (l *Lang) UnmarshalText(text []byte) error {
	switch string(text) {
	case goName:
		*l = Go
	case scriptName:
		*l = Script
	default:
		return fmt.Errorf("%w: %q", ErrUnknownLang, text)
	}

	return nil
}

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

// ScriptProduction reports whether file is a non-test script source that measurements read.
func ScriptProduction(cfg config.Config, file string) bool {
	k, ok := Classify(cfg, file)

	return ok && k.Lang == Script && !k.Test
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
