package semantics

import (
	"context"

	"testing"
)

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

// Review finding #4: ordinary parentheses around the parameter root do not
// change the write-through target. Selector and index writes rooted at
// parenthesized pointer/map/slice parameters must still be detected.
func TestGoMutatesInput_ParenthesizedRootWrites(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		wantName string
		evidence string
	}{
		{
			name: "selector on parenthesized pointer parameter",
			source: `package main

type Config struct {
	Name string
}

func f(cfg *Config) {
	(cfg).Name = "x"
}
`,
			wantName: "f:cfg",
			evidence: "(cfg).Name",
		},
		{
			name: "index on parenthesized slice parameter",
			source: `package main

func g(items []int) {
	(items)[0] = 1
}
`,
			wantName: "g:items",
			evidence: "(items)[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_featuresPart3Test_81(t, tt)
		})
	}
}

func TestGoMutatesInput_UpdateExpressionsMutateParameterRoots(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		wantName string
		evidence string
	}{
		{
			name: "pointer selector increment",
			source: `package main

type Config struct {
	Count int
}

func f(cfg *Config) {
	cfg.Count++
}
`,
			wantName: "f:cfg",
			evidence: "cfg.Count",
		},
		{
			name: "slice index decrement",
			source: `package main

func f(items []int) {
	items[0]--
}
`,
			wantName: "f:items",
			evidence: "items[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_featuresPart3Test_134(t, tt)
		})
	}
}

// AC-3.4: max_nesting_depth is the maximum depth of nested block nodes
// within any single function/method body, where the body's own top-level
// block counts as depth 1. This fixture nests: func body (block, depth 1)
// -> if consequence (block, depth 2) -> for body (block, depth 3) -> if
// consequence (block, depth 4). The innermost if body is empty, so it adds
// no further depth.
func TestMetrics_MaxNestingDepthWithinFunctionBody(t *testing.T) {
	source := []byte(`package main

func f() {
	if true {
		for {
			if true {
			}
		}
	}
}
`)
	root, closeTree := mustParseGo(t, source)
	defer closeTree()

	metrics, _ := computeGoFeatures(root, source)

	if metrics.MaxNestingDepth != 4 {
		t.Errorf("computeGoFeatures MaxNestingDepth for 4-deep nested blocks %q: got %d, want %d", source, metrics.MaxNestingDepth, 4)
	}
}
