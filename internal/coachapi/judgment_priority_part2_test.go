package coachapi

import (
	"testing"

	"github.com/lousy-agents/coach/internal/rubrics"
)

func TestResolveMaxHiddenMutationJudgments(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in   int
		want int
	}{
		{0, DefaultMaxHiddenMutationJudgments},
		{16, 16},
		{8, 8},
		{-1, -1},
		{-99, -1},
	}
	for _, tc := range cases {
		if got := resolveMaxHiddenMutationJudgments(tc.in); got != tc.want {
			t.Errorf("resolveMaxHiddenMutationJudgments(%d)=%d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestPrioritizeJudgmentCandidates_UnlimitedAndUnderCap(t *testing.T) {
	t.Parallel()
	cands := []rubrics.PackCandidate{
		{FindingRef: "1", Path: "b.go", StartRow: 1},
		{FindingRef: "2", Path: "a.go", StartRow: 1},
		{FindingRef: "3", Path: "a.go", StartRow: 2},
	}
	all, omitted := PrioritizeJudgmentCandidates(cands, -1)
	if omitted != 0 || len(all) != 3 {
		t.Fatalf("unlimited: selected=%d omitted=%d", len(all), omitted)
	}
	under, omitted := PrioritizeJudgmentCandidates(cands, 10)
	if omitted != 0 || len(under) != 3 {
		t.Fatalf("under cap: selected=%d omitted=%d", len(under), omitted)
	}
}

func refs(cands []rubrics.PackCandidate) []string {
	out := make([]string, len(cands))
	for i, c := range cands {
		out[i] = c.FindingRef
	}
	return out
}
