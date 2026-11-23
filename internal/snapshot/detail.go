package snapshot

import (
	"cmp"
	"slices"

	"github.com/mocoarow/repohealth/internal/complexity"
	"github.com/mocoarow/repohealth/internal/config"
	"github.com/mocoarow/repohealth/internal/coverage"
	"github.com/mocoarow/repohealth/internal/crossarea"
	"github.com/mocoarow/repohealth/internal/godeps"
	"github.com/mocoarow/repohealth/internal/hotspot"
	"github.com/mocoarow/repohealth/internal/size"
	"github.com/mocoarow/repohealth/internal/source"
)

// Detail is the full result of the latest snapshot, cut to the top DetailLimit entries where lists grow with the code.
type Detail struct {
	Header

	CrossArea    crossarea.Result     `json:"crossArea"`
	Hotspots     []hotspot.File       `json:"hotspots"`
	Functions    []Function           `json:"functions"`
	AreaImports  []godeps.AreaImports `json:"areaImports"`
	Stability    []godeps.Stability   `json:"stability"`
	Coverage     coverage.Result      `json:"coverage"`
	LargestFiles []size.File          `json:"largestFiles"`
}

// Function is the complexity of one function and its language.
type Function struct {
	Lang source.Lang `json:"lang"`

	complexity.Function
}

// Detailed shapes m into a Detail; cfg selects the packages whose stability is reported.
func Detailed(cfg config.Config, h Header, m Measurements) Detail {
	hotspots := hotspot.Rank(slices.Concat(m.Go, m.TS), m.Churn)

	return Detail{
		Header:       h,
		CrossArea:    crossarea.Result{Months: nonNil(m.CrossArea.Months), Pairs: nonNil(m.CrossArea.Pairs)},
		Hotspots:     nonNil(hotspots[:min(len(hotspots), DetailLimit)]),
		Functions:    nonNil(slices.Concat(topFunctions(source.Go, m.Go), topFunctions(source.Script, m.TS))),
		AreaImports:  nonNil(m.Graph.AreaImports()),
		Stability:    nonNil(m.Graph.Stability(cfg)),
		Coverage:     coverage.Result{Reports: nonNil(m.Coverage.Reports), Layers: nonNil(m.Coverage.Layers)},
		LargestFiles: largestFiles(m.Sizes),
	}
}

func topFunctions(lang source.Lang, funcs []complexity.Function) []Function {
	sorted := slices.SortedFunc(slices.Values(funcs), func(a, b complexity.Function) int {
		return cmp.Or(cmp.Compare(b.Cognit, a.Cognit), cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line))
	})

	top := make([]Function, 0, min(len(sorted), DetailLimit))
	for _, f := range sorted[:min(len(sorted), DetailLimit)] {
		top = append(top, Function{Lang: lang, Function: f})
	}

	return top
}

func largestFiles(files []size.File) []size.File {
	production := []size.File{}

	for _, f := range files {
		if !f.Test {
			production = append(production, f)
		}
	}

	slices.SortFunc(production, func(a, b size.File) int {
		return cmp.Or(cmp.Compare(b.Lines, a.Lines), cmp.Compare(a.Path, b.Path))
	})

	return production[:min(len(production), DetailLimit)]
}
