package codesignal

import (
	"testing"
)

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
