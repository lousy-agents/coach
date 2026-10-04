package codesignal

import (
	"testing"
)

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
