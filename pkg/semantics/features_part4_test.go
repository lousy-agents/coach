package semantics

import (
	"reflect"

	"testing"
)

// AC-3.7: Finding's grammar-node facts (Kind, Name, Location) are required
// and must stay first, in this order; the remaining fields are optional
// coaching metadata (Confidence, Evidence, Recommendation, SuggestedSkill)
// used by findings like "mutates_input" and omitted via omitempty for
// findings that don't set them. Checked structurally via reflection rather
// than by code review, so a future field addition fails this test loudly.
func TestFinding_StructCarriesOnlyDataFields(t *testing.T) {
	typ := reflect.TypeOf(Finding{})

	wantFields := []string{"Kind", "Name", "Location", "Confidence", "Evidence", "Recommendation", "SuggestedSkill"}
	if typ.NumField() != len(wantFields) {
		t.Fatalf("Finding field count: got %d fields, want exactly %d (%v)", typ.NumField(), len(wantFields), wantFields)
	}
	for i, want := range wantFields {
		if got := typ.Field(i).Name; got != want {
			t.Errorf("Finding field %d: got %q, want %q", i, got, want)
		}
	}
}

// hasFinding reports whether findings contains a Finding with the given
// kind and name.
func hasFinding(findings []Finding, kind, name string) bool {
	for _, f := range findings {
		if f.Kind == kind && f.Name == name {
			return true
		}
	}
	return false
}

// mutatesInputFinding returns a pointer to the first "mutates_input" Finding
// in findings matching name, or nil if there is none.
func mutatesInputFinding(findings []Finding, name string) *Finding {
	for i := range findings {
		if findings[i].Kind == "mutates_input" && findings[i].Name == name {
			return &findings[i]
		}
	}
	return nil
}

// countMutatesInputFindings reports how many "mutates_input" findings with
// the given name are present in findings.
func countMutatesInputFindings(findings []Finding, name string) int {
	n := 0
	for _, f := range findings {
		if f.Kind == "mutates_input" && f.Name == name {
			n++
		}
	}
	return n
}

// Review finding #1: assigning directly through a pointer-typed parameter's
// dereference (*cfg = Config{...}) mutates the caller-visible value and must
// emit mutates_input, even though the assignment target is not a selector.
func TestGoMutatesInput_DirectPointerDereferenceAssignment(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	*cfg = Config{Name: "x"}
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	got := mutatesInputFinding(findings, "f:cfg")
	if got == nil {
		t.Fatalf("computeGoFeatures findings for direct pointer dereference assignment %q: want a mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
	if got.Evidence != "*cfg" {
		t.Errorf("mutates_input Evidence: got %q, want %q", got.Evidence, "*cfg")
	}
}
