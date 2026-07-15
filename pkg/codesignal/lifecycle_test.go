package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func sig(ruleID, path, subject, evidence string, startRow, startCol uint) Signal {
	return Signal{
		RuleID:   ruleID,
		Path:     path,
		Subject:  subject,
		Evidence: evidence,
		Location: semantics.Location{StartRow: startRow, StartCol: startCol},
	}
}

func lifecyclesFor(t *testing.T, signals []Signal, subject string, lifecycle Lifecycle) int {
	t.Helper()
	count := 0
	for _, s := range signals {
		if s.Subject == subject && s.Lifecycle == lifecycle {
			count++
		}
	}
	return count
}

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

func TestLifecycle_ClassifyFileSignals_NewKeyWithBasePresentIsIntroduced(t *testing.T) {
	head := []Signal{
		sig("r", "f.go", "Y", "ev", 1, 0),
		sig("r", "f.go", "Y", "ev", 2, 0),
		sig("r", "f.go", "Y", "ev", 3, 0),
	}

	got := classifyFileSignals(true, head, nil, "unknown")

	if len(got) != 3 {
		t.Fatalf("classifyFileSignals result length: got %d, want 3: %+v", len(got), got)
	}
	if n := lifecyclesFor(t, got, "Y", "introduced"); n != 3 {
		t.Errorf("introduced signals: got %d, want 3: %+v", n, got)
	}
	if n := lifecyclesFor(t, got, "Y", "unknown"); n != 0 {
		t.Errorf("unknown signals: got %d, want 0: %+v", n, got)
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

func TestLifecycle_ClassifyFileSignals_NoBaseAtAllMeansUnknown(t *testing.T) {
	head := []Signal{
		sig("r", "f.go", "W", "ev", 1, 0),
		sig("r", "f.go", "W", "ev", 2, 0),
	}

	got := classifyFileSignals(false, head, nil, "unknown")

	if len(got) != 2 {
		t.Fatalf("classifyFileSignals result length: got %d, want 2: %+v", len(got), got)
	}
	if n := lifecyclesFor(t, got, "W", "unknown"); n != 2 {
		t.Errorf("unknown signals: got %d, want 2: %+v", n, got)
	}
	if n := lifecyclesFor(t, got, "W", "resolved"); n != 0 {
		t.Errorf("resolved signals: got %d, want 0: %+v", n, got)
	}
}

func TestClassifyFileSignals_BaselineLifecycle(t *testing.T) {
	head := []Signal{
		sig("r", "f.go", "V", "ev", 1, 0),
		sig("r", "f.go", "V", "ev", 2, 0),
	}

	got := classifyFileSignals(false, head, nil, "baseline")

	if len(got) != 2 {
		t.Fatalf("classifyFileSignals result length: got %d, want 2: %+v", len(got), got)
	}
	if n := lifecyclesFor(t, got, "V", "baseline"); n != 2 {
		t.Errorf("baseline signals: got %d, want 2: %+v", n, got)
	}
	if n := lifecyclesFor(t, got, "V", "unknown"); n != 0 {
		t.Errorf("unknown signals: got %d, want 0: %+v", n, got)
	}
}

func TestLifecycle_ClassifyFileSignals_FingerprintIsLocationIndependent(t *testing.T) {
	a := sig("r", "f.go", "Moved", "ev", 1, 0)
	b := sig("r", "f.go", "Moved", "ev", 50, 0)

	gotA := classifyFileSignals(false, []Signal{a}, nil, "unknown")
	gotB := classifyFileSignals(false, []Signal{b}, nil, "unknown")

	if len(gotA) != 1 || len(gotB) != 1 {
		t.Fatalf("classifyFileSignals result lengths: got %d and %d, want 1 and 1", len(gotA), len(gotB))
	}

	if gotA[0].Fingerprint != gotB[0].Fingerprint {
		t.Errorf("Fingerprint must be location-independent for a solo occurrence: got %q and %q", gotA[0].Fingerprint, gotB[0].Fingerprint)
	}
	if gotA[0].Fingerprint == "" {
		t.Errorf("Fingerprint must not be empty")
	}
	if gotA[0].ID == gotB[0].ID {
		t.Errorf("ID must be location-sensitive: got the same ID %q for signals at different locations", gotA[0].ID)
	}
}

func TestLifecycle_ClassifyFileSignals_DoesNotMutateInputSlices(t *testing.T) {
	head := []Signal{sig("r", "f.go", "M", "ev", 1, 0)}
	base := []Signal{sig("r", "f.go", "M", "ev", 1, 0)}

	headCopy := append([]Signal(nil), head...)
	baseCopy := append([]Signal(nil), base...)

	classifyFileSignals(true, head, base, "unknown")

	if head[0].Lifecycle != headCopy[0].Lifecycle || head[0].Fingerprint != headCopy[0].Fingerprint {
		t.Errorf("classifyFileSignals must not mutate the caller's headSignals slice in place: got %+v, want %+v", head[0], headCopy[0])
	}
	if base[0].Lifecycle != baseCopy[0].Lifecycle || base[0].Fingerprint != baseCopy[0].Fingerprint {
		t.Errorf("classifyFileSignals must not mutate the caller's baseSignals slice in place: got %+v, want %+v", base[0], baseCopy[0])
	}
}
