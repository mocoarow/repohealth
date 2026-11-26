package report_test

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

// offsetSec is an offset no time zone of a CI machine is likely to share, so a time keeps its offset in the CSV.
const offsetSec = 5*60*60 + 45*60

func zone() *time.Location {
	return time.FixedZone("", offsetSec)
}

func headerOf(rev string, takenAt, committedAt time.Time) snapshot.Header {
	return snapshot.Header{
		SchemaVersion: snapshot.SchemaVersion,
		TakenAt:       takenAt,
		Rev:           rev,
		CommittedAt:   committedAt,
		Tools:         snapshot.Tools{Repohealth: "1.2.3", Biome: "2.3.4"},
	}
}

func fixedHeader(rev string) snapshot.Header {
	return headerOf(rev, time.Date(2026, 3, 4, 5, 6, 7, 0, zone()), time.Date(2026, 3, 3, 10, 11, 12, 0, zone()))
}

func areaImports() []godeps.AreaImports {
	return []godeps.AreaImports{{From: "api", To: "core", Imports: 6}}
}

func stability() []godeps.Stability {
	return []godeps.Stability{{Dir: "internal/api", Afferent: 2, Efferent: 3, Instability: 0.6}}
}

func coverageReports() []coverage.Report {
	return []coverage.Report{{File: "coverage/lcov.info", ModTime: time.Date(2026, 3, 2, 1, 2, 3, 0, zone()), Skipped: 7}}
}

// fixedSummary returns a Summary whose every value differs from its zero value.
func fixedSummary(t *testing.T) snapshot.Summary {
	t.Helper()

	return snapshot.Summary{
		Header: fixedHeader("abc123"),
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

// skippedSummary returns a Summary whose only row is coverage_skipped.
func skippedSummary(t *testing.T, h snapshot.Header, skipped int) snapshot.Summary {
	t.Helper()

	return snapshot.Summary{
		Header:          h,
		Areas:           []snapshot.Area{},
		AreaImports:     []godeps.AreaImports{},
		Stability:       []godeps.Stability{},
		CoverageReports: []coverage.Report{{File: "lcov.info", ModTime: time.Time{}, Skipped: skipped}},
	}
}

// areaOf returns an area whose values are derived from n, so each area and snapshot differs.
func areaOf(name string, n int) snapshot.Area {
	return snapshot.Area{
		Name:     name,
		Coverage: snapshot.Lines{Covered: n, Lines: n + 1},
		Size: snapshot.Size{
			Go: snapshot.LangSize{Lines: 100 * n, TestLines: 50 * n},
			TS: snapshot.LangSize{Lines: 30 * n, TestLines: 20 * n},
		},
		Complexity: snapshot.Complexity{
			Go: snapshot.GoComplexity{Functions: 10 * n, CycloMax: n + 1, CognitSum: 7 * n, CognitMax: n + 2, CognitOverLimit: n},
			TS: snapshot.TSComplexity{Functions: 3 * n, CognitSum: 5 * n, CognitMax: n + 3, CognitOverLimit: n + 4},
		},
	}
}

func summaryOf(h snapshot.Header, areas ...snapshot.Area) snapshot.Summary {
	return snapshot.Summary{
		Header:          h,
		Areas:           areas,
		AreaImports:     areaImports(),
		Stability:       stability(),
		CoverageReports: coverageReports(),
	}
}

func committedOn(rev string, day int) snapshot.Header {
	return headerOf(rev, time.Date(2026, 3, day, 12, 0, 0, 0, zone()), time.Date(2026, 3, day, 9, 0, 0, 0, zone()))
}

// trendSummaries returns three snapshots: "gone" leaves after the first, "web" joins in the last and has no coverage lines.
func trendSummaries(t *testing.T) []snapshot.Summary {
	t.Helper()

	web := areaOf("web", 3)
	web.Coverage = snapshot.Lines{Covered: 0, Lines: 0}

	return []snapshot.Summary{
		summaryOf(committedOn("r2", 2), areaOf("api", 2)),
		summaryOf(fixedHeader("abc123"), areaOf("api", 4), web),
		summaryOf(committedOn("r1", 1), areaOf("api", 1), areaOf("gone", 9)),
	}
}

// fixedDetail returns a Detail whose every value differs from its zero value.
func fixedDetail(t *testing.T) snapshot.Detail {
	t.Helper()

	return snapshot.Detail{
		Header: fixedHeader("abc123"),
		CrossArea: crossarea.Result{
			Months: []crossarea.Month{
				{Period: "2026-02", PRs: 8, CrossArea: 3, Groups: []crossarea.GroupCount{
					{Group: "backend", CrossArea: 2}, {Group: "web", CrossArea: 1},
				}},
				{Period: "2026-03", PRs: 5, CrossArea: 4, Groups: []crossarea.GroupCount{
					{Group: "backend", CrossArea: 3}, {Group: "web", CrossArea: 4},
				}},
			},
			Pairs: []crossarea.Pair{{A: "api", B: "core", PRs: 4}},
		},
		Hotspots: []hotspot.File{
			{Path: "internal/api/handler.go", Churn: 9, RecentChurn: 5, SumCognit: 17, MaxCognit: 12, Score: 153},
		},
		Functions: []snapshot.Function{
			{
				Lang:     source.Go,
				Function: complexity.Function{File: "internal/api/handler.go", Line: 42, Name: "Handle", Cyclo: 6, Cognit: 12},
			},
			{
				Lang:     source.Script,
				Function: complexity.Function{File: "web/app.ts", Line: 7, Name: "render", Cyclo: 9, Cognit: 4},
			},
		},
		AreaImports: areaImports(),
		Stability:   stability(),
		Coverage: coverage.Result{
			Reports: coverageReports(),
			Layers:  []coverage.Layer{{Area: "api", Layer: "internal", Covered: 8, Lines: 10}},
		},
		LargestFiles: []size.File{{Path: "web/app.ts", Area: "web", Lang: source.Script, Test: false, Lines: 321}},
	}
}
