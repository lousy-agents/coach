package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestLifecycle_ClassifyFileSignals_MoreBaseThanHeadResolvesExcess(t *testing.T) {
	base := []Signal{
		sig("r", "f.go", "X", "ev", 1, 0),
		sig("r", "f.go", "X", "ev", 2, 0),
		sig("r", "f.go", "X", "ev", 3, 0),
	}
	head := []Signal{
		sig("r", "f.go", "X", "ev", 1, 0),
		sig("r", "f.go", "X", "ev", 2, 0),
	}

	got := classifyFileSignals(true, head, base, "unknown")

	if len(got) != 3 {
		t.Fatalf("classifyFileSignals result length: got %d, want 3: %+v", len(got), got)
	}
	if n := lifecyclesFor(t, got, "X", "existing"); n != 2 {
		t.Errorf("existing signals: got %d, want 2: %+v", n, got)
	}
	if n := lifecyclesFor(t, got, "X", "resolved"); n != 1 {
		t.Errorf("resolved signals: got %d, want 1: %+v", n, got)
	}
	if n := lifecyclesFor(t, got, "X", "introduced"); n != 0 {
		t.Errorf("introduced signals: got %d, want 0: %+v", n, got)
	}
	if n := lifecyclesFor(t, got, "X", "unknown"); n != 0 {
		t.Errorf("unknown signals: got %d, want 0: %+v", n, got)
	}

	for _, s := range got {
		if s.Lifecycle == "resolved" && s.Fingerprint == "" {
			t.Errorf("resolved signal must have a non-empty Fingerprint: %+v", s)
		}
	}
}

func TestLifecycle_ClassifyFileSignals_ExcessBeyondNonZeroBaseIsUnknown(t *testing.T) {
	base := []Signal{
		sig("r", "f.go", "Z", "ev", 1, 0),
	}
	head := []Signal{
		sig("r", "f.go", "Z", "ev", 1, 0),
		sig("r", "f.go", "Z", "ev", 2, 0),
		sig("r", "f.go", "Z", "ev", 3, 0),
	}

	got := classifyFileSignals(true, head, base, "unknown")

	if n := lifecyclesFor(t, got, "Z", "existing"); n != 1 {
		t.Errorf("existing signals: got %d, want 1: %+v", n, got)
	}
	if n := lifecyclesFor(t, got, "Z", "unknown"); n != 2 {
		t.Errorf("unknown signals: got %d, want 2: %+v", n, got)
	}
	if n := lifecyclesFor(t, got, "Z", "introduced"); n != 0 {
		t.Errorf("introduced signals: got %d, want 0: %+v", n, got)
	}
	if n := lifecyclesFor(t, got, "Z", "resolved"); n != 0 {
		t.Errorf("resolved signals: got %d, want 0: %+v", n, got)
	}
}

func sig(ruleID, path, subject, evidence string, startRow, startCol uint) Signal {
	return Signal{
		RuleID:   ruleID,
		Path:     path,
		Subject:  subject,
		Evidence: evidence,
		Location: semantics.Location{StartRow: startRow, StartCol: startCol},
	}
}
