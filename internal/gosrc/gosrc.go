package gosrc

import (
	"path"
	"strings"

	"github.com/mocoarow/repohealth/internal/config"
)

// Counted reports whether the Go measurements read file.
func Counted(cfg config.Config, file string) bool {
	if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") || ignoredDir(path.Dir(file)) {
		return false
	}

	_, ok := cfg.AreaOf(file)

	return ok
}

func ignoredDir(dir string) bool {
	for elem := range strings.SplitSeq(dir, "/") {
		if elem == "testdata" || (elem != "." && (strings.HasPrefix(elem, ".") || strings.HasPrefix(elem, "_"))) {
			return true
		}
	}

	return false
}
