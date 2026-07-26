package coachapi

import (
	"strconv"
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

func TestPrioritizeJudgmentCandidates_RoundRobinAcrossPaths(t *testing.T) {
	t.Parallel()
	// 20 + 3 + 3 + 2 with cap 16 → 8 hot + 3 + 3 + 2 cold (not 16 from hot).
	var cands []rubrics.PackCandidate
	for i := 0; i < 20; i++ {
		cands = append(cands, rubrics.PackCandidate{
			FindingRef: "hot-" + itoa(i),
			Path:       "hot.go",
			StartRow:   i + 1,
			Severity:   "medium",
			Confidence: "medium",
		})
	}
	for i := 0; i < 3; i++ {
		cands = append(cands, rubrics.PackCandidate{
			FindingRef: "a-" + itoa(i),
			Path:       "cold_a.go",
			StartRow:   i + 1,
			Severity:   "medium",
			Confidence: "medium",
		})
	}
	for i := 0; i < 3; i++ {
		cands = append(cands, rubrics.PackCandidate{
			FindingRef: "b-" + itoa(i),
			Path:       "cold_b.go",
			StartRow:   i + 1,
			Severity:   "medium",
			Confidence: "medium",
		})
	}
	for i := 0; i < 2; i++ {
		cands = append(cands, rubrics.PackCandidate{
			FindingRef: "c-" + itoa(i),
			Path:       "cold_c.go",
			StartRow:   i + 1,
			Severity:   "medium",
			Confidence: "medium",
		})
	}

	selected, omitted := PrioritizeJudgmentCandidates(cands, 16)
	if len(selected) != 16 {
		t.Fatalf("selected=%d, want 16", len(selected))
	}
	if omitted != 12 {
		t.Fatalf("omitted=%d, want 12", omitted)
	}
	counts := map[string]int{}
	for _, c := range selected {
		counts[c.Path]++
	}
	if counts["hot.go"] != 8 || counts["cold_a.go"] != 3 || counts["cold_b.go"] != 3 || counts["cold_c.go"] != 2 {
		t.Fatalf("path spread=%v, want hot=8 cold_a=3 cold_b=3 cold_c=2", counts)
	}
}

func TestPrioritizeJudgmentCandidates_WithinPathSeverityThenConfidence(t *testing.T) {
	t.Parallel()
	cands := []rubrics.PackCandidate{
		{FindingRef: "low", Path: "a.go", StartRow: 1, Severity: "low", Confidence: "high"},
		{FindingRef: "high-med", Path: "a.go", StartRow: 2, Severity: "high", Confidence: "medium"},
		{FindingRef: "high-high", Path: "a.go", StartRow: 3, Severity: "high", Confidence: "high"},
		{FindingRef: "med", Path: "a.go", StartRow: 4, Severity: "medium", Confidence: "medium"},
	}
	selected, omitted := PrioritizeJudgmentCandidates(cands, 4)
	if omitted != 0 {
		t.Fatalf("omitted=%d, want 0", omitted)
	}
	want := []string{"high-high", "high-med", "med", "low"}
	for i, ref := range want {
		if selected[i].FindingRef != ref {
			t.Errorf("selected[%d]=%q, want %q (order=%v)", i, selected[i].FindingRef, ref, refs(selected))
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

func itoa(i int) string { return strconv.Itoa(i) }

func refs(cands []rubrics.PackCandidate) []string {
	out := make([]string, len(cands))
	for i, c := range cands {
		out[i] = c.FindingRef
	}
	return out
}
