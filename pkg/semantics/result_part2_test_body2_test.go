package semantics

import (
	"testing"
)

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
