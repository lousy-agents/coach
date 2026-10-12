package semantics

import (
	"testing"
)

// GitHub issue #179: an act call whose first argument's source text differs
// from the Stat call's first argument must not be flagged -- the paths are
// not provably the same value.
func TestGoTOCTOU_PathTextMismatch_NoFinding(t *testing.T) {
	source := []byte(`package main

import "os"

func f(path, other string) {
	if _, err := os.Stat(path); err == nil {
		os.Open(other)
	}
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)
	if got := toctouFindingsOf(findings); len(got) != 0 {
		t.Errorf("mismatched path text %q: got %d toctou_check_then_act findings, want 0: %+v", source, len(got), got)
	}
}

// GitHub issue #179: if the Stat call's error result is ignored (assigned
// to "_", never used as the if's own condition), there is no nil-comparison
// gate on that error and no Finding must be emitted.
func TestGoTOCTOU_StatErrorIgnored_NoFinding(t *testing.T) {
	source := []byte(`package main

import "os"

func f(path string, ready bool) {
	if _, _ := os.Stat(path); ready {
		os.Open(path)
	}
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)
	if got := toctouFindingsOf(findings); len(got) != 0 {
		t.Errorf("Stat error result ignored %q: got %d toctou_check_then_act findings, want 0: %+v", source, len(got), got)
	}
}
