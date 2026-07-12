package codesignal

import "testing"

func TestFingerprint_NormalizePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "already clean", path: "pkg/foo/bar.go", want: "pkg/foo/bar.go"},
		{name: "leading dot slash", path: "./pkg/foo/bar.go", want: "pkg/foo/bar.go"},
		{name: "leading slash", path: "/pkg/foo/bar.go", want: "pkg/foo/bar.go"},
		{name: "multiple leading slashes", path: "///pkg/foo/bar.go", want: "pkg/foo/bar.go"},
		{name: "backslashes", path: `pkg\foo\bar.go`, want: "pkg/foo/bar.go"},
		{name: "leading dot slash with backslashes", path: `.\pkg\foo\bar.go`, want: "pkg/foo/bar.go"},
		{name: "empty", path: "", want: ""},
		{name: "only leading dot slash", path: "./", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizePath(tt.path)
			if got != tt.want {
				t.Errorf("normalizePath(%q): got %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestFingerprint_NormalizeEvidence(t *testing.T) {
	tests := []struct {
		name     string
		evidence string
		want     string
	}{
		{name: "already clean", evidence: "x = y", want: "x = y"},
		{name: "leading whitespace", evidence: "   x = y", want: "x = y"},
		{name: "trailing whitespace", evidence: "x = y   ", want: "x = y"},
		{name: "leading and trailing whitespace with newlines", evidence: "\n\t x = y \n", want: "x = y"},
		{name: "empty", evidence: "", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeEvidence(tt.evidence)
			if got != tt.want {
				t.Errorf("normalizeEvidence(%q): got %q, want %q", tt.evidence, got, tt.want)
			}
		})
	}
}

func TestFingerprint_ComputeFingerprint_Deterministic(t *testing.T) {
	a := computeFingerprint("state.hidden_input_mutation", "pkg/foo.go", "Foo", "x.y = 1", 0)
	b := computeFingerprint("state.hidden_input_mutation", "pkg/foo.go", "Foo", "x.y = 1", 0)

	if a != b {
		t.Errorf("computeFingerprint with identical inputs: got %q and %q, want equal", a, b)
	}
	if a == "" {
		t.Errorf("computeFingerprint must not return an empty string")
	}
}

func TestFingerprint_ComputeSignalID_Deterministic(t *testing.T) {
	a := computeSignalID("state.hidden_input_mutation", "pkg/foo.go", "Foo", "x.y = 1", 4, 2, 0)
	b := computeSignalID("state.hidden_input_mutation", "pkg/foo.go", "Foo", "x.y = 1", 4, 2, 0)

	if a != b {
		t.Errorf("computeSignalID with identical inputs: got %q and %q, want equal", a, b)
	}
	if a == "" {
		t.Errorf("computeSignalID must not return an empty string")
	}
}

func TestFingerprint_ComputeFingerprint_DifferentOrdinalDiffers(t *testing.T) {
	a := computeFingerprint("state.hidden_input_mutation", "pkg/foo.go", "Foo", "x.y = 1", 0)
	b := computeFingerprint("state.hidden_input_mutation", "pkg/foo.go", "Foo", "x.y = 1", 1)

	if a == b {
		t.Errorf("computeFingerprint with different ordinals must differ; both were %q", a)
	}
}

func TestFingerprint_ComputeSignalID_DifferentOrdinalDiffers(t *testing.T) {
	a := computeSignalID("state.hidden_input_mutation", "pkg/foo.go", "Foo", "x.y = 1", 4, 2, 0)
	b := computeSignalID("state.hidden_input_mutation", "pkg/foo.go", "Foo", "x.y = 1", 4, 2, 1)

	if a == b {
		t.Errorf("computeSignalID with different ordinals must differ; both were %q", a)
	}
}

// TestFingerprint_ComputeFingerprint_LengthPrefixingPreventsFieldBoundaryCollisions
// constructs an adversarial pair of inputs that would collide under naive
// string concatenation without length-prefixing ("ab"+"c" == "a"+"bc") and
// asserts the fingerprints differ.
func TestFingerprint_ComputeFingerprint_LengthPrefixingPreventsFieldBoundaryCollisions(t *testing.T) {
	a := computeFingerprint("ab", "c", "subject", "evidence", 0)
	b := computeFingerprint("a", "bc", "subject", "evidence", 0)

	if a == b {
		t.Errorf("computeFingerprint(%q, %q, ...) and computeFingerprint(%q, %q, ...) collided: both were %q; length-prefixing must prevent field-boundary collisions", "ab", "c", "a", "bc", a)
	}
}

// TestFingerprint_ComputeSignalID_LengthPrefixingPreventsFieldBoundaryCollisions
// is the computeSignalID analog of the fingerprint collision test above.
func TestFingerprint_ComputeSignalID_LengthPrefixingPreventsFieldBoundaryCollisions(t *testing.T) {
	a := computeSignalID("ab", "c", "subject", "evidence", 0, 0, 0)
	b := computeSignalID("a", "bc", "subject", "evidence", 0, 0, 0)

	if a == b {
		t.Errorf("computeSignalID(%q, %q, ...) and computeSignalID(%q, %q, ...) collided: both were %q; length-prefixing must prevent field-boundary collisions", "ab", "c", "a", "bc", a)
	}
}

// TestFingerprint_ComputeFingerprint_EmbeddedNullByteDoesNotCollide proves a
// field containing an embedded null byte doesn't collapse two
// otherwise-distinct inputs onto the same fingerprint.
func TestFingerprint_ComputeFingerprint_EmbeddedNullByteDoesNotCollide(t *testing.T) {
	a := computeFingerprint("rule", "path", "sub\x00ject", "evidence", 0)
	b := computeFingerprint("rule", "path", "subject", "evidence", 0)

	if a == b {
		t.Errorf("computeFingerprint with an embedded null byte in subject must not collide with the same fields minus the null byte: both were %q", a)
	}
}
