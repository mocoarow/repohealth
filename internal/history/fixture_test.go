package history_test

import (
	"testing"
	"time"

	"github.com/mocoarow/repohealth/internal/complexity"
	"github.com/mocoarow/repohealth/internal/coverage"
	"github.com/mocoarow/repohealth/internal/crossarea"
	"github.com/mocoarow/repohealth/internal/godeps"
	"github.com/mocoarow/repohealth/internal/hotspot"
	"github.com/mocoarow/repohealth/internal/size"
	"github.com/mocoarow/repohealth/internal/snapshot"
	"github.com/mocoarow/repohealth/internal/source"
)

// offsetSec is an offset no time zone of a CI machine is likely to share, so a decoded time keeps its offset.
const offsetSec = 5*60*60 + 45*60

func header(rev string) snapshot.Header {
	zone := time.FixedZone("", offsetSec)

	return snapshot.Header{
		SchemaVersion: snapshot.SchemaVersion,
		TakenAt:       time.Date(2026, 3, 4, 5, 6, 7, 0, zone),
		Rev:           rev,
		CommittedAt:   time.Date(2026, 3, 3, 10, 11, 12, 0, zone),
		Tools:         snapshot.Tools{Repohealth: "1.2.3", Biome: "2.3.4"},
	}
}

func areaImports() []godeps.AreaImports {
	return []godeps.AreaImports{{From: "api", To: "core", Imports: 6}}
}

func stability() []godeps.Stability {
	return []godeps.Stability{{Dir: "internal/api", Afferent: 2, Efferent: 3, Instability: 0.6}}
}

func coverageReports() []coverage.Report {
	zone := time.FixedZone("", offsetSec)

	return []coverage.Report{{File: "coverage/lcov.info", ModTime: time.Date(2026, 3, 2, 1, 2, 3, 0, zone), Skipped: 7}}
}

// summaryOf returns a Summary whose every field differs from its zero value.
func summaryOf(t *testing.T, rev string) snapshot.Summary {
	t.Helper()

	return snapshot.Summary{
		Header: header(rev),
		Areas: []snapshot.Area{{
			Name:     "api",
			Coverage: snapshot.Lines{Covered: 31, Lines: 32},
			Size: snapshot.Size{
				Go: snapshot.LangSize{Lines: 101, TestLines: 102},
				TS: snapshot.LangSize{Lines: 103, TestLines: 104},
			},
			Complexity: snapshot.Complexity{
				Go: snapshot.GoComplexity{Functions: 11, CycloMax: 12, CognitSum: 13, CognitMax: 14, CognitOverLimit: 15},
				TS: snapshot.TSComplexity{Functions: 21, CognitSum: 22, CognitMax: 23, CognitOverLimit: 24},
			},
		}},
		AreaImports:     areaImports(),
		Stability:       stability(),
		CoverageReports: coverageReports(),
	}
}

// detailOf returns a Detail whose every field differs from its zero value.
func detailOf(t *testing.T, rev string) snapshot.Detail {
	t.Helper()

	return snapshot.Detail{
		Header: header(rev),
		CrossArea: crossarea.Result{
			Months: []crossarea.Month{{
				Period: "2026-02", PRs: 8, CrossArea: 3,
				Groups: []crossarea.GroupCount{{Group: "backend", CrossArea: 2}},
			}},
			Pairs: []crossarea.Pair{{A: "api", B: "core", PRs: 4}},
		},
		Hotspots: []hotspot.File{{
			Path: "internal/api/handler.go", Churn: 9, RecentChurn: 5, SumCognit: 17, MaxCognit: 12, Score: 153,
		}},
		Functions: []snapshot.Function{{
			Lang: source.Go,
			Function: complexity.Function{
				File: "internal/api/handler.go", Line: 42, Name: "Handle", Cyclo: 6, Cognit: 12,
			},
		}},
		AreaImports: areaImports(),
		Stability:   stability(),
		Coverage: coverage.Result{
			Reports: coverageReports(),
			Layers:  []coverage.Layer{{Area: "api", Layer: "internal", Covered: 8, Lines: 10}},
		},
		LargestFiles: []size.File{{Path: "web/app.ts", Area: "web", Lang: source.Script, Test: true, Lines: 321}},
	}
}
