package codesignal

import (
	"testing"
)

func body_fingerprintTest_24(t *testing.T, tt struct {
	name string
	path string
	want string
}) {
	got := normalizePath(tt.path)
	if got != tt.want {
		t.Errorf("normalizePath(%q): got %q, want %q", tt.path, got, tt.want)
	}
}

func body_fingerprintTest_47(t *testing.T, tt struct {
	name     string
	evidence string
	want     string
}) {
	got := normalizeEvidence(tt.evidence)
	if got != tt.want {
		t.Errorf("normalizeEvidence(%q): got %q, want %q", tt.evidence, got, tt.want)
	}
}
