package semantics

import (
	"testing"
)

// GitHub issue #179: a nested Stat-gated if on the same path must dedupe to
// a single Finding on the act call, mirroring the TS detector's nested-guard
// dedup rule (see checkGoTOCTOUCheckThenAct's doc comment).
func TestGoTOCTOU_NestedGuardsDedupeToOneFinding(t *testing.T) {
	source := []byte(`package main

import "os"

func f(path string) {
	if _, err := os.Stat(path); err == nil {
		if _, err := os.Stat(path); err == nil {
			os.Open(path)
		}
	}
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)
	if got := toctouFindingsOf(findings); len(got) != 1 {
		t.Errorf("nested Stat guards on same path %q: got %d toctou_check_then_act findings, want 1: %+v", source, len(got), got)
	}
}
