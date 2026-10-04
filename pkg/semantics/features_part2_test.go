package semantics

import (
	"strings"
	"testing"
)

// Story 1/3: a selector write through a syntactically pointer-typed
// parameter (cfg.Name = "x", cfg *Config) must emit exactly one
// "mutates_input" Finding, with Name "<func>:<param>", Location pointing at
// the mutation expression (not the function declaration), and the coaching
// metadata fields set as specified.
func TestGoMutatesInput_PointerSelectorWrite(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	cfg.Name = "x"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	got := mutatesInputFinding(findings, "f:cfg")
	if got == nil {
		t.Fatalf("computeGoFeatures findings for %q: want a mutates_input finding named %q, got %+v", source, "f:cfg", findings)
	}
	if got.Evidence != "cfg.Name" {
		t.Errorf("mutates_input Evidence: got %q, want %q", got.Evidence, "cfg.Name")
	}
	if got.Confidence != "medium" {
		t.Errorf("mutates_input Confidence: got %q, want %q", got.Confidence, "medium")
	}
	if got.Recommendation == "" {
		t.Errorf("mutates_input Recommendation: got empty, want a non-empty recommendation")
	}
	if got.SuggestedSkill != "refactor-hidden-mutation" {
		t.Errorf("mutates_input SuggestedSkill: got %q, want %q", got.SuggestedSkill, "refactor-hidden-mutation")
	}
	wantExprStart := uint(strings.Index(string(source), `cfg.Name = "x"`))
	if got.Location.StartByte != wantExprStart {
		t.Errorf("mutates_input Location.StartByte: got %d, want %d (start of mutation expression, not the function declaration)", got.Location.StartByte, wantExprStart)
	}
}

// AC-3.5: a function_declaration whose name matches ^New([A-Z0-9_]|$) must
// emit a "constructor_func" Finding named after the function. NewFoo and the
// bare New both match; Newton must not, since 't' is neither uppercase,
// a digit, underscore, nor end-of-string.
func TestFindings_ConstructorFuncMatchesNewPrefixButNotNewton(t *testing.T) {
	source := []byte(`package main

func NewFoo() {}

func New() {}

func Newton() {}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	constructorNames := map[string]bool{}
	for _, f := range findings {
		if f.Kind == "constructor_func" {
			constructorNames[f.Name] = true
		}
	}

	if !constructorNames["NewFoo"] {
		t.Errorf("computeGoFeatures findings for %q: want a constructor_func finding named %q, got %+v", source, "NewFoo", findings)
	}
	if !constructorNames["New"] {
		t.Errorf("computeGoFeatures findings for %q: want a constructor_func finding named %q, got %+v", source, "New", findings)
	}
	if constructorNames["Newton"] {
		t.Errorf("computeGoFeatures findings for %q: want no constructor_func finding named %q (does not match ^New([A-Z0-9_]|$)), got %+v", source, "Newton", findings)
	}
}
