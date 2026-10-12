package codesignal

import (
	"testing"
)

func TestFingerprint_ComputeFingerprint_LengthPrefixingPreventsFieldBoundaryCollisions(t *testing.T) {
	a := computeFingerprint("ab", "c", "subject", "evidence", 0)
	b := computeFingerprint("a", "bc", "subject", "evidence", 0)

	if a == b {
		t.Errorf("computeFingerprint(%q, %q, ...) and computeFingerprint(%q, %q, ...) collided: both were %q; length-prefixing must prevent field-boundary collisions", "ab", "c", "a", "bc", a)
	}
}

func TestFingerprint_ComputeSignalID_LengthPrefixingPreventsFieldBoundaryCollisions(t *testing.T) {
	a := computeSignalID("ab", "c", "subject", "evidence", 0, 0, 0)
	b := computeSignalID("a", "bc", "subject", "evidence", 0, 0, 0)

	if a == b {
		t.Errorf("computeSignalID(%q, %q, ...) and computeSignalID(%q, %q, ...) collided: both were %q; length-prefixing must prevent field-boundary collisions", "ab", "c", "a", "bc", a)
	}
}

func TestFingerprint_ComputeFingerprint_EmbeddedNullByteDoesNotCollide(t *testing.T) {
	a := computeFingerprint("rule", "path", "sub\x00ject", "evidence", 0)
	b := computeFingerprint("rule", "path", "subject", "evidence", 0)

	if a == b {
		t.Errorf("computeFingerprint with an embedded null byte in subject must not collide with the same fields minus the null byte: both were %q", a)
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
