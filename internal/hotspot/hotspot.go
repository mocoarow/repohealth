package hotspot

import (
	"cmp"
	"slices"

	"github.com/mocoarow/repohealth/internal/complexity"
)

// minCognit leaves out trivial functions, which Biome never reports, so Go and TypeScript count alike.
const minCognit = 2

// File is one file's hotspot score.
type File struct {
	Path      string
	Churn     int
	SumCognit int
	MaxCognit int
	Score     int
}

// Rank scores every file that has a function of at least minCognit.
func Rank(funcs []complexity.Function, churn map[string]int) []File {
	byPath := map[string]File{}

	for _, f := range funcs {
		if f.Cognit < minCognit {
			continue
		}

		file := byPath[f.File]
		byPath[f.File] = File{
			Path:      f.File,
			Churn:     churn[f.File],
			SumCognit: file.SumCognit + f.Cognit,
			MaxCognit: max(file.MaxCognit, f.Cognit),
			Score:     0,
		}
	}

	files := make([]File, 0, len(byPath))
	for _, file := range byPath {
		files = append(files, File{
			Path:      file.Path,
			Churn:     file.Churn,
			SumCognit: file.SumCognit,
			MaxCognit: file.MaxCognit,
			Score:     file.Churn * file.SumCognit,
		})
	}

	slices.SortFunc(files, func(a, b File) int {
		return cmp.Or(cmp.Compare(b.Score, a.Score), cmp.Compare(a.Path, b.Path))
	})

	return files
}
