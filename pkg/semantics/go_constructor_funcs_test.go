package semantics

import (
	"testing"
)

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
