package semantics

import (
	"errors"
	"fmt"

	"testing"
)

// Result.ReactComponents must stay nil (and thus be omitted from JSON via
// omitempty) when AnalyzeBytes finds no React client component candidates:
// Go sources, plain TS/TSX without candidacy, and syntax-error partial
// results.
func TestResult_ReactComponentsEmpty(t *testing.T) {
	a := mustNewAnalyzer(t)

	tests := []struct {
		name       string
		input      FileInput
		wantStatus ParseStatus
	}{
		{
			name: "go ok",
			input: FileInput{
				Path:     "main.go",
				Language: LanguageGo,
				Content:  []byte("package main\n\nfunc main() {}\n"),
			},
			wantStatus: ParseStatus("ok"),
		},
		{
			name: "typescript ok",
			input: FileInput{
				Path:     "main.ts",
				Language: LanguageTypeScript,
				Content:  []byte("const x: number = 1;\n"),
			},
			wantStatus: ParseStatus("ok"),
		},
		{
			name: "tsx ok",
			input: FileInput{
				Path:     "App.tsx",
				Language: LanguageTSX,
				Content:  []byte("const App = () => <div>hi</div>;\n"),
			},
			wantStatus: ParseStatus("ok"),
		},
		{
			name: "tsx syntax error",
			input: FileInput{
				Path:     "broken.tsx",
				Language: LanguageTSX,
				Content:  []byte("const x = ;"),
			},
			wantStatus: ParseStatus("syntax_errors"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_resultTest_63(t, a, tt)
		})
	}
}

// AC-2.3: errors.As must extract a *SyntaxError from a wrapping error chain,
// and its Issues must match what was originally constructed.
func TestSyntaxError_AsExtractsIssues(t *testing.T) {
	issues := []SyntaxIssue{
		{Kind: "error", Location: Location{StartByte: 1, EndByte: 2}},
		{Kind: "missing", Location: Location{StartByte: 3, EndByte: 3}},
	}
	original := &SyntaxError{Issues: issues}
	wrapped := fmt.Errorf("analyzing file: %w", original)

	var got *SyntaxError
	if !errors.As(wrapped, &got) {
		t.Fatalf("AC-2.3: errors.As(%v, &SyntaxError{}) = false, want true", wrapped)
	}

	if len(got.Issues) != len(issues) {
		t.Fatalf("AC-2.3: extracted SyntaxError.Issues length: got %d, want %d", len(got.Issues), len(issues))
	}
	for i, want := range issues {
		if got.Issues[i] != want {
			t.Errorf("AC-2.3: extracted SyntaxError.Issues[%d]: got %+v, want %+v", i, got.Issues[i], want)
		}
	}
}

// goldenOkResult builds the small, hand-legible fixture used by the AC-4.4
// golden test for a clean parse: one import, one finding, one zero-score
// Cognitive Complexity record, no syntax errors. Shape matches a reachable
// AnalyzeBytes "ok" result (ParseStatus "ok" carries Imports/Metrics/Findings
// and per-function cognitive_complexity, never SyntaxErrors).
func goldenOkResult() Result {
	loc := Location{
		StartByte: 30, EndByte: 45,
		StartRow: 3, StartCol: 0,
		EndRow: 3, EndCol: 15,
	}
	return Result{
		Path:        "example.go",
		Language:    LanguageGo,
		ParseStatus: ParseStatus("ok"),
		Imports: []ImportFeature{
			{
				Path: "fmt",
				Location: Location{
					StartByte: 20, EndByte: 25,
					StartRow: 2, StartCol: 1,
					EndRow: 2, EndCol: 6,
				},
			},
		},
		Metrics: StructuralMetrics{
			Ifs: 0, Fors: 0, ExprSwitches: 0, TypeSwitches: 0,
			Selects: 0, Functions: 1, Methods: 0, MaxNestingDepth: 0,
		},
		Findings: []Finding{
			{
				Kind:     "constructor_func",
				Name:     "NewThing",
				Location: loc,
			},
		},
		CognitiveComplexity: []FunctionCognitiveComplexity{
			{
				Name:     "NewThing",
				Kind:     "function",
				Location: loc,
				Score:    0,
			},
		},
	}
}

// goldenSyntaxErrorResult builds the small, hand-legible fixture used by the
// AC-4.4 golden test for a syntax-error parse: one syntax error, zero-valued
// Metrics, no Imports/Findings. This is the other state AnalyzeBytes can
// actually return (ParseStatus "syntax_errors" always carries SyntaxErrors
// and zero-valued/omitted everything else -- see analyzer.go's HasError
// branch). Kept as a separate fixture from goldenOkResult rather than
// combined into one, since no real Result ever has both SyntaxErrors and
// populated Imports/Metrics/Findings at once.
func goldenSyntaxErrorResult() Result {
	return Result{
		Path:        "broken.go",
		Language:    LanguageGo,
		ParseStatus: ParseStatus("syntax_errors"),
		SyntaxErrors: []SyntaxIssue{
			{
				Kind: "error",
				Location: Location{
					StartByte: 10, EndByte: 11,
					StartRow: 1, StartCol: 0,
					EndRow: 1, EndCol: 1,
				},
			},
		},
	}
}

// goldenReactComponentsResult locks the additive react_components JSON
// field family (Story 4 / epic #139) under the same golden discipline as
// cognitive_complexity: one minimal, hand-legible record with every nested
// fact slice present so snake_case tags and omitempty presence stay frozen.
func goldenReactComponentsResult() Result {
	compLoc := Location{
		StartByte: 40, EndByte: 200,
		StartRow: 2, StartCol: 0,
		EndRow: 12, EndCol: 1,
	}
	bindLoc := Location{
		StartByte: 60, EndByte: 90,
		StartRow: 3, StartCol: 2,
		EndRow: 3, EndCol: 32,
	}
	return Result{
		Path:        "WorkspacePage.tsx",
		Language:    LanguageTSX,
		ParseStatus: ParseStatus("ok"),
		Metrics: StructuralMetrics{
			Functions: 1,
		},
		ReactComponents: []ReactComponentFacts{
			{
				Name:       "WorkspacePage",
				Location:   compLoc,
				ClientKind: "use_client_directive",
				UseState: []ReactUseStateBinding{
					{Binding: "activeView", Setter: "setActiveView", Location: bindLoc},
				},
				CoordinatedTransitions: []ReactCoordinatedTransition{
					{
						Name:            "<anonymous>",
						Kind:            "effect",
						Location:        Location{StartByte: 100, EndByte: 140, StartRow: 5, StartCol: 2, EndRow: 7, EndCol: 4},
						UpdatedBindings: []string{"activeView", "filterText"},
					},
				},
				WorkspaceBranches: []ReactWorkspaceBranch{
					{Label: "list", Location: Location{StartByte: 150, EndByte: 160, StartRow: 8, StartCol: 8, EndRow: 8, EndCol: 18}},
				},
				ImperativeUI: []ReactImperativeUICall{
					{API: "getElementById", Location: Location{StartByte: 70, EndByte: 99, StartRow: 4, StartCol: 2, EndRow: 4, EndCol: 31}},
				},
				SharedPanelDeps: []ReactSharedPanelDep{
					{Name: "selectedId", Panels: []string{"DetailPanel", "ListPanel"}},
				},
			},
		},
	}
}
