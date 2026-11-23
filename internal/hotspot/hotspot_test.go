package hotspot_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/mocoarow/repohealth/internal/complexity"
	"github.com/mocoarow/repohealth/internal/hotspot"
)

func fn(file string, cognit int) complexity.Function {
	return complexity.Function{File: file, Line: 1, Name: "f", Cyclo: 1, Cognit: cognit}
}

func Test_Rank_shouldScoreChurnTimesSumOfCognit_whenFileHasSeveralFunctions(t *testing.T) {
	t.Parallel()

	// given
	funcs := []complexity.Function{fn("a.go", 3), fn("a.go", 5)}
	churn := map[string]int{"a.go": 4}

	// when
	got := hotspot.Rank(funcs, churn)

	// then
	assert.Equal(t, []hotspot.File{{Path: "a.go", Churn: 4, SumCognit: 8, MaxCognit: 5, Score: 32}}, got)
}

func Test_Rank_shouldLeaveOutFunction_whenCognitIsBelowTwo(t *testing.T) {
	t.Parallel()

	// given
	funcs := []complexity.Function{fn("a.go", 3), fn("a.go", 1), fn("a.go", 0)}
	churn := map[string]int{"a.go": 2}

	// when
	got := hotspot.Rank(funcs, churn)

	// then
	assert.Equal(t, []hotspot.File{{Path: "a.go", Churn: 2, SumCognit: 3, MaxCognit: 3, Score: 6}}, got)
}

func Test_Rank_shouldLeaveOutFile_whenNoFunctionReachesTwo(t *testing.T) {
	t.Parallel()

	// given
	funcs := []complexity.Function{fn("trivial.go", 1)}
	churn := map[string]int{"trivial.go": 5}

	// when
	got := hotspot.Rank(funcs, churn)

	// then
	assert.Empty(t, got)
}

func Test_Rank_shouldOrderByScoreThenPath_whenFilesDiffer(t *testing.T) {
	t.Parallel()

	// given
	funcs := []complexity.Function{fn("c.go", 2), fn("b.go", 2), fn("a.go", 2), fn("d.go", 10)}
	churn := map[string]int{"a.go": 3, "b.go": 3, "c.go": 3, "d.go": 1}

	// when
	got := hotspot.Rank(funcs, churn)

	// then
	assert.Equal(t, []string{"d.go", "a.go", "b.go", "c.go"}, paths(got))
}

func Test_Rank_shouldScoreZero_whenFileHasNoChurn(t *testing.T) {
	t.Parallel()

	// given
	funcs := []complexity.Function{fn("new.go", 7)}
	churn := map[string]int{"other.go": 2}

	// when
	got := hotspot.Rank(funcs, churn)

	// then
	assert.Equal(t, []hotspot.File{{Path: "new.go", Churn: 0, SumCognit: 7, MaxCognit: 7, Score: 0}}, got)
}

func Test_Rank_shouldReturnNoFiles_whenThereAreNoFunctions(t *testing.T) {
	t.Parallel()

	// given
	churn := map[string]int{"a.go": 2}

	// when
	got := hotspot.Rank(nil, churn)

	// then
	assert.Empty(t, got)
}

func paths(files []hotspot.File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Path)
	}

	return out
}
