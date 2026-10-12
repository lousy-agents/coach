package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestTOCTOUCheckThenAct_FieldsPassThroughFromFinding(t *testing.T) {
	finding := semantics.Finding{
		Kind:           "toctou_check_then_act",
		Name:           "readConfig",
		Location:       semantics.Location{StartRow: 4, EndRow: 4},
		Evidence:       "if (existsSync(path)) { readFileSync(path) }",
		SuggestedSkill: "find-bugs",
	}

	signal := newTOCTOUCheckThenActSignal("f.ts", finding)

	if signal.RuleID != "security.toctou_check_then_act" {
		t.Errorf("Signal.RuleID: got %q, want %q", signal.RuleID, "security.toctou_check_then_act")
	}
	if signal.RuleVersion != "1" {
		t.Errorf("Signal.RuleVersion: got %q, want %q", signal.RuleVersion, "1")
	}
	if signal.Kind != "toctou_check_then_act" {
		t.Errorf("Signal.Kind: got %q, want %q", signal.Kind, "toctou_check_then_act")
	}
	if signal.Category != Category("security") {
		t.Errorf("Signal.Category: got %q, want %q", signal.Category, "security")
	}
	if signal.Severity != Severity("medium") {
		t.Errorf("Signal.Severity: got %q, want %q", signal.Severity, "medium")
	}
	if signal.Path != "f.ts" {
		t.Errorf("Signal.Path: got %q, want %q", signal.Path, "f.ts")
	}
	if signal.Subject != finding.Name {
		t.Errorf("Signal.Subject: got %q, want %q", signal.Subject, finding.Name)
	}
	if signal.Location != finding.Location {
		t.Errorf("Signal.Location: got %+v, want %+v", signal.Location, finding.Location)
	}
	if signal.Evidence != finding.Evidence {
		t.Errorf("Signal.Evidence: got %q, want %q", signal.Evidence, finding.Evidence)
	}
	if signal.SuggestedSkill != finding.SuggestedSkill {
		t.Errorf("Signal.SuggestedSkill: got %q, want %q", signal.SuggestedSkill, finding.SuggestedSkill)
	}
	if signal.Provenance != (Provenance{Producer: "semantics", FindingKind: "toctou_check_then_act"}) {
		t.Errorf("Signal.Provenance: got %+v, want %+v", signal.Provenance, Provenance{Producer: "semantics", FindingKind: "toctou_check_then_act"})
	}
}
