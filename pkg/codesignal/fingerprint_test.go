package codesignal

import (
	"testing"
)

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
			expectNormalizedPath(t, tt.path, tt.want)
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
			expectNormalizedEvidence(t, tt.evidence, tt.want)
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

func expectNormalizedPath(t *testing.T, path, want string) {
	t.Helper()
	if got := normalizePath(path); got != want {
		t.Errorf("normalizePath(%q): got %q, want %q", path, got, want)
	}
}

func expectNormalizedEvidence(t *testing.T, evidence, want string) {
	t.Helper()
	if got := normalizeEvidence(evidence); got != want {
		t.Errorf("normalizeEvidence(%q): got %q, want %q", evidence, got, want)
	}
}
