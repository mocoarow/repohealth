package crossarea

import (
	"cmp"
	"maps"
	"slices"
	"time"

	"github.com/mocoarow/repohealth/internal/config"
)

const (
	monthLayout = "2006-01"
	crossing    = 2
)

// Change is one merged pull request.
type Change struct {
	MergedAt time.Time
	Files    []string
}

// Result is the cross-area measurement.
type Result struct {
	Months []Month `json:"months"`
	Pairs  []Pair  `json:"pairs"`
}

// Month counts the pull requests merged in one month.
type Month struct {
	Period    string       `json:"period"`
	PRs       int          `json:"prs"`
	CrossArea int          `json:"crossArea"`
	Groups    []GroupCount `json:"groups"`
}

// GroupCount counts the pull requests that touch two or more areas of one group.
type GroupCount struct {
	Group     string `json:"group"`
	CrossArea int    `json:"crossArea"`
}

// Pair counts the pull requests that touch both areas.
type Pair struct {
	A   string `json:"a"`
	B   string `json:"b"`
	PRs int    `json:"prs"`
}

type pairKey struct {
	a string
	b string
}

// Measure counts the pull requests per month and per pair of areas.
func Measure(cfg config.Config, changes []Change) Result {
	months := map[string]Month{}
	pairs := map[pairKey]int{}

	for _, c := range changes {
		areas := touchedAreas(cfg, c.Files)
		if len(areas) == 0 {
			continue
		}

		key := c.MergedAt.Format(monthLayout)

		months[key] = addChange(months[key], key, cfg.Groups, areas)

		for _, p := range pairsOf(areas) {
			pairs[p]++
		}
	}

	return Result{Months: sortedMonths(months), Pairs: sortedPairs(pairs)}
}

func touchedAreas(cfg config.Config, files []string) []string {
	seen := map[string]bool{}

	for _, f := range files {
		if area, ok := cfg.AreaOf(f); ok {
			seen[area] = true
		}
	}

	return slices.Sorted(maps.Keys(seen))
}

func addChange(m Month, key string, groups []config.Group, areas []string) Month {
	counts := make([]GroupCount, 0, len(groups))

	for i, g := range groups {
		count := 0
		if i < len(m.Groups) {
			count = m.Groups[i].CrossArea
		}

		if countIn(areas, g.Areas) >= crossing {
			count++
		}

		counts = append(counts, GroupCount{Group: g.Name, CrossArea: count})
	}

	crossArea := m.CrossArea
	if len(areas) >= crossing {
		crossArea++
	}

	return Month{Period: key, PRs: m.PRs + 1, CrossArea: crossArea, Groups: counts}
}

func countIn(areas, members []string) int {
	n := 0

	for _, a := range areas {
		if slices.Contains(members, a) {
			n++
		}
	}

	return n
}

func pairsOf(areas []string) []pairKey {
	var pairs []pairKey

	for i := range areas {
		for _, b := range areas[i+1:] {
			pairs = append(pairs, pairKey{a: areas[i], b: b})
		}
	}

	return pairs
}

func sortedMonths(months map[string]Month) []Month {
	return slices.SortedFunc(maps.Values(months), func(a, b Month) int {
		return cmp.Compare(a.Period, b.Period)
	})
}

func sortedPairs(counts map[pairKey]int) []Pair {
	pairs := make([]Pair, 0, len(counts))
	for k, n := range counts {
		pairs = append(pairs, Pair{A: k.a, B: k.b, PRs: n})
	}

	slices.SortFunc(pairs, func(x, y Pair) int {
		return cmp.Or(cmp.Compare(y.PRs, x.PRs), cmp.Compare(x.A, y.A), cmp.Compare(x.B, y.B))
	})

	return pairs
}
