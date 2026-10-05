package semantics

import (
	"testing"
)

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

func checkGoldenReactComponentsRoundTrip(t *testing.T, r Result) {
	t.Helper()
	if r.ParseStatus != ParseStatus("ok") {
		t.Errorf("AC-4.4: golden react_components Result.ParseStatus: got %q, want %q", r.ParseStatus, "ok")
	}
	if r.Language != LanguageTSX {
		t.Errorf("AC-4.4: golden react_components Result.Language: got %q, want %q", r.Language, LanguageTSX)
	}
	if len(r.ReactComponents) != 1 {
		t.Fatalf("AC-4.4: golden react_components length: got %d, want 1", len(r.ReactComponents))
	}
	checkGoldenWorkspacePageRecord(t, r.ReactComponents[0])
}

func checkGoldenWorkspacePageRecord(t *testing.T, rec ReactComponentFacts) {
	t.Helper()
	if rec.Name != "WorkspacePage" || rec.ClientKind != "use_client_directive" {
		t.Errorf("AC-4.4: golden react_components[0] name/client_kind: got %q/%q", rec.Name, rec.ClientKind)
	}
	if len(rec.UseState) != 1 || rec.UseState[0].Binding != "activeView" || rec.UseState[0].Setter != "setActiveView" {
		t.Errorf("AC-4.4: golden react_components[0].use_state: got %+v", rec.UseState)
	}
	if len(rec.CoordinatedTransitions) != 1 || rec.CoordinatedTransitions[0].Kind != "effect" {
		t.Errorf("AC-4.4: golden react_components[0].coordinated_transitions: got %+v", rec.CoordinatedTransitions)
	}
	if len(rec.WorkspaceBranches) != 1 || rec.WorkspaceBranches[0].Label != "list" {
		t.Errorf("AC-4.4: golden react_components[0].workspace_branches: got %+v", rec.WorkspaceBranches)
	}
	if len(rec.ImperativeUI) != 1 || rec.ImperativeUI[0].API != "getElementById" {
		t.Errorf("AC-4.4: golden react_components[0].imperative_ui: got %+v", rec.ImperativeUI)
	}
	if len(rec.SharedPanelDeps) != 1 || rec.SharedPanelDeps[0].Name != "selectedId" {
		t.Errorf("AC-4.4: golden react_components[0].shared_panel_deps: got %+v", rec.SharedPanelDeps)
	}
}
