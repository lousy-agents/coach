package semantics

import (
	"testing"
)

func body_toctouGoTest_22(t *testing.T, tt struct {
	name    string
	actExpr string
}) {
	source := []byte(`package main

import "os"

func f(path string) {
	if _, err := os.Stat(path); err == nil {
		` + tt.actExpr + `
	}
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)
	got := toctouFindingsOf(findings)
	if len(got) != 1 {
		t.Fatalf("os.Stat gating %s %q: got %d toctou_check_then_act findings, want 1: %+v", tt.name, source, len(got), findings)
	}
	if got[0].Confidence != "medium" {
		t.Errorf("os.Stat gating %s: Confidence = %q, want %q", tt.name, got[0].Confidence, "medium")
	}
	if got[0].SuggestedSkill != "find-bugs" {
		t.Errorf("os.Stat gating %s: SuggestedSkill = %q, want %q", tt.name, got[0].SuggestedSkill, "find-bugs")
	}
}
