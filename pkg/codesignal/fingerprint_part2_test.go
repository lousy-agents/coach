package codesignal

import (
	"testing"
)

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
