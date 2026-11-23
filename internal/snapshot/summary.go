package snapshot

import (
	"github.com/mocoarow/repohealth/internal/complexity"
	"github.com/mocoarow/repohealth/internal/config"
	"github.com/mocoarow/repohealth/internal/coverage"
	"github.com/mocoarow/repohealth/internal/godeps"
	"github.com/mocoarow/repohealth/internal/size"
	"github.com/mocoarow/repohealth/internal/source"
)

// Summary is the per-area digest of one snapshot, kept for every snapshot in the history.
type Summary struct {
	Header

	Areas           []Area               `json:"areas"`
	AreaImports     []godeps.AreaImports `json:"areaImports"`
	Stability       []godeps.Stability   `json:"stability"`
	CoverageReports []coverage.Report    `json:"coverageReports"`
}

// Area is the digest of one configured area.
type Area struct {
	Name       string     `json:"name"`
	Coverage   Lines      `json:"coverage"`
	Size       Size       `json:"size"`
	Complexity Complexity `json:"complexity"`
}

// Lines counts covered lines among the lines a coverage report instruments.
type Lines struct {
	Covered int `json:"covered"`
	Lines   int `json:"lines"`
}

// Size is the line count of an area per language.
type Size struct {
	Go LangSize `json:"go"`
	TS LangSize `json:"ts"`
}

// LangSize counts the lines of production and of test files.
type LangSize struct {
	Lines     int `json:"lines"`
	TestLines int `json:"testLines"`
}

// Complexity is the complexity of an area per language; the languages are measured differently and never summed.
type Complexity struct {
	Go GoComplexity `json:"go"`
	TS TSComplexity `json:"ts"`
}

// GoComplexity sums every hand-written Go function.
type GoComplexity struct {
	Functions       int `json:"functions"`
	CycloMax        int `json:"cycloMax"`
	CognitSum       int `json:"cognitSum"`
	CognitMax       int `json:"cognitMax"`
	CognitOverLimit int `json:"cognitOverLimit"`
}

// TSComplexity sums the script functions Biome reports, which are those of cognitive complexity 2 or more.
type TSComplexity struct {
	Functions       int `json:"functions"`
	CognitSum       int `json:"cognitSum"`
	CognitMax       int `json:"cognitMax"`
	CognitOverLimit int `json:"cognitOverLimit"`
}

// Summarize digests m per area, in the order of cfg.Areas, including areas with nothing measured.
func Summarize(cfg config.Config, h Header, m Measurements) Summary {
	areas := make([]Area, 0, len(cfg.Areas))
	for _, a := range cfg.Areas {
		areas = append(areas, Area{
			Name:     a.Name,
			Coverage: coverageOf(a.Name, m.Coverage.Layers),
			Size:     sizeOf(a.Name, m.Sizes),
			Complexity: Complexity{
				Go: goComplexityOf(inArea(cfg, a.Name, m.Go)),
				TS: tsComplexityOf(inArea(cfg, a.Name, m.TS)),
			},
		})
	}

	return Summary{
		Header:          h,
		Areas:           areas,
		AreaImports:     nonNil(m.Graph.AreaImports()),
		Stability:       nonNil(m.Graph.Stability(cfg)),
		CoverageReports: nonNil(m.Coverage.Reports),
	}
}

func coverageOf(area string, layers []coverage.Layer) Lines {
	sum := Lines{Covered: 0, Lines: 0}

	for _, l := range layers {
		if l.Area == area {
			sum = Lines{Covered: sum.Covered + l.Covered, Lines: sum.Lines + l.Lines}
		}
	}

	return sum
}

func sizeOf(area string, files []size.File) Size {
	sum := Size{Go: LangSize{Lines: 0, TestLines: 0}, TS: LangSize{Lines: 0, TestLines: 0}}

	for _, f := range files {
		if f.Area != area {
			continue
		}

		switch f.Lang {
		case source.Go:
			sum.Go = addLines(sum.Go, f)
		case source.Script:
			sum.TS = addLines(sum.TS, f)
		}
	}

	return sum
}

func addLines(s LangSize, f size.File) LangSize {
	if f.Test {
		return LangSize{Lines: s.Lines, TestLines: s.TestLines + f.Lines}
	}

	return LangSize{Lines: s.Lines + f.Lines, TestLines: s.TestLines}
}

func inArea(cfg config.Config, area string, funcs []complexity.Function) []complexity.Function {
	var found []complexity.Function

	for _, f := range funcs {
		if a, ok := cfg.AreaOf(f.File); ok && a == area {
			found = append(found, f)
		}
	}

	return found
}

func goComplexityOf(funcs []complexity.Function) GoComplexity {
	c := GoComplexity{Functions: len(funcs), CycloMax: 0, CognitSum: 0, CognitMax: 0, CognitOverLimit: 0}

	for _, f := range funcs {
		c.CycloMax = max(c.CycloMax, f.Cyclo)
		c.CognitSum += f.Cognit
		c.CognitMax = max(c.CognitMax, f.Cognit)
		c.CognitOverLimit += overLimit(f)
	}

	return c
}

func tsComplexityOf(funcs []complexity.Function) TSComplexity {
	c := TSComplexity{Functions: len(funcs), CognitSum: 0, CognitMax: 0, CognitOverLimit: 0}

	for _, f := range funcs {
		c.CognitSum += f.Cognit
		c.CognitMax = max(c.CognitMax, f.Cognit)
		c.CognitOverLimit += overLimit(f)
	}

	return c
}

func overLimit(f complexity.Function) int {
	if f.Cognit > CognitLimit {
		return 1
	}

	return 0
}
