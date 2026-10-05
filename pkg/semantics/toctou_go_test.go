package semantics

import (
	"testing"
)

// GitHub issue #179: each of the five documented act calls
// (Open/OpenFile/Remove/RemoveAll/ReadFile) must be detected when gated
// behind a matching os.Stat check.
func TestGoTOCTOU_EachActCallName(t *testing.T) {
	tests := []goTOCTOUActCallCase{
		{"Open", `os.Open(path)`},
		{"OpenFile", `os.OpenFile(path, os.O_RDONLY, 0)`},
		{"Remove", `os.Remove(path)`},
		{"RemoveAll", `os.RemoveAll(path)`},
		{"ReadFile", `os.ReadFile(path)`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expectGoTOCTOUFindingForActCall(t, tt)
		})
	}
}

// toctouFindingsOf filters findings down to "toctou_check_then_act" kind,
// mirroring toctouFindings' role in the acceptance test package for this
// package's white-box tests.
func toctouFindingsOf(findings []Finding) []Finding {
	var out []Finding
	for _, f := range findings {
		if f.Kind == "toctou_check_then_act" {
			out = append(out, f)
		}
	}
	return out
}

// GitHub issue #179 (Story 3, CWE-367): os.Lstat is an equally valid check
// call alongside os.Stat.
func TestGoTOCTOU_LstatVariant(t *testing.T) {
	source := []byte(`package main

import "os"

func f(path string) {
	if _, err := os.Lstat(path); err == nil {
		os.Open(path)
	}
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)
	got := toctouFindingsOf(findings)
	if len(got) != 1 {
		t.Fatalf("os.Lstat check-then-act %q: got %d toctou_check_then_act findings, want 1: %+v", source, len(got), findings)
	}
	if got[0].Name != "path" {
		t.Errorf("os.Lstat check-then-act %q: Name = %q, want %q", source, got[0].Name, "path")
	}
}

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

// GitHub issue #179: the plain-assignment initializer form (`=`, node kind
// assignment_statement) must be handled identically to the short
// variable-declaration form (`:=`).
func TestGoTOCTOU_PlainAssignmentInitializer(t *testing.T) {
	source := []byte(`package main

import "os"

func f(path string) {
	var fi os.FileInfo
	var err error
	if fi, err = os.Stat(path); err == nil {
		_ = fi
		os.Open(path)
	}
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)
	got := toctouFindingsOf(findings)
	if len(got) != 1 {
		t.Fatalf("plain-assignment initializer %q: got %d toctou_check_then_act findings, want 1: %+v", source, len(got), findings)
	}
}
