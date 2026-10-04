package codesignal

import (
	"testing"
)

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
