package baseline

import (
	"strconv"
	"testing"

	"github.com/lousy-agents/coach/internal/rubrics"
)

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

func itoa(i int) string { return strconv.Itoa(i) }
