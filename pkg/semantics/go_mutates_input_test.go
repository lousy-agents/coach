package semantics

import (
	"context"
	"testing"
)

// AC-5: multiple writes through the same parameter at distinct source
// locations must produce one finding per distinct location, and writing to
// the exact same expression location must never be double-counted.
func TestGoMutatesInput_DuplicateWritesDeduped(t *testing.T) {
	source := []byte(`package main

type Config struct {
	Name string
	Age  int
}

func f(cfg *Config) {
	cfg.Name = "x"
	cfg.Age = 1
	cfg.Name = "y"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	got := countMutatesInputFindings(findings, "f:cfg")
	if got != 3 {
		t.Errorf("computeGoFeatures findings for %q: got %d mutates_input findings named %q (one per distinct mutation-expression location), want 3; findings=%+v", source, got, "f:cfg", findings)
	}

	seenLocations := map[uint]bool{}
	for _, f := range findings {
		if f.Kind != "mutates_input" || f.Name != "f:cfg" {
			continue
		}
		if seenLocations[f.Location.StartByte] {
			t.Errorf("computeGoFeatures findings for %q: duplicate mutates_input finding at the same Location.StartByte %d", source, f.Location.StartByte)
		}
		seenLocations[f.Location.StartByte] = true
	}
}

// AC-6: computeGoFeatures is only reached from AnalyzeBytes on a clean parse
// (analyzer.go returns a partial Result on root.HasError() before ever
// calling spec.computeFeatures), so a file with syntax errors can never
// produce mutates_input findings. Verified at the AnalyzeBytes level,
// matching the existing syntax-error test pattern used elsewhere in this
// package.
func TestGoMutatesInput_SyntaxErrorEmitsNoFindings(t *testing.T) {
	source := []byte(`package main

func f(cfg *Config) {
	cfg.Name =
}
`)
	a := mustNewAnalyzer(t)
	result, err := a.AnalyzeBytes(context.Background(), FileInput{
		Path:     "f.go",
		Language: LanguageGo,
		Content:  source,
	})
	if err == nil {
		t.Fatalf("AnalyzeBytes for syntactically invalid source %q: got nil err, want a syntax error", source)
	}
	if result == nil {
		t.Fatalf("AnalyzeBytes for syntactically invalid source %q: got nil result, want a partial Result", source)
	}
	if result.ParseStatus != ParseStatus("syntax_errors") {
		t.Errorf("AnalyzeBytes for syntactically invalid source %q: ParseStatus = %q, want %q", source, result.ParseStatus, "syntax_errors")
	}
	if len(result.Findings) != 0 {
		t.Errorf("AnalyzeBytes for syntactically invalid source %q: Findings = %+v, want empty", source, result.Findings)
	}
}

// A pointer-typed receiver mutated through a selector is not a declared
// parameter (the issue explicitly scopes this feature to parameters, not
// receivers), so a method mutating only its receiver must not emit a
// mutates_input finding.
func TestGoMutatesInput_ReceiverMutationIsNotAParameter(t *testing.T) {
	source := []byte(`package main

type T struct {
	Name string
}

func (t *T) Method() {
	t.Name = "x"
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	_, findings := computeGoFeatures(root, source)

	for _, f := range findings {
		if f.Kind == "mutates_input" {
			t.Errorf("computeGoFeatures findings for receiver-only mutation %q: want no mutates_input findings (receiver is not a parameter), got %+v", source, findings)
		}
	}
}
