package semantics

import (
	"testing"
)

type tsTOCTOUActCallCase struct {
	name        string
	actCallText string
}

func expectTSTOCTOUFindingForActCall(t *testing.T, tt tsTOCTOUActCallCase) {
	source := "function f(p: string) {\n\tif (existsSync(p)) {\n\t\t" + tt.actCallText + ";\n\t}\n}\n"
	root, closeTree := mustParseTS(t, []byte(source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(source))

	var got []Finding
	for _, f := range findings {
		if f.Kind == "toctou_check_then_act" {
			got = append(got, f)
		}
	}
	if len(got) != 1 {
		t.Fatalf("computeTSFeatures for %q: got %d toctou_check_then_act findings (%+v), want exactly 1", source, len(got), findings)
	}
	f := got[0]
	if f.Kind != "toctou_check_then_act" {
		t.Errorf("Finding.Kind = %q, want %q", f.Kind, "toctou_check_then_act")
	}
	if f.Confidence != "medium" {
		t.Errorf("Finding.Confidence = %q, want %q", f.Confidence, "medium")
	}
	if f.SuggestedSkill != "find-bugs" {
		t.Errorf("Finding.SuggestedSkill = %q, want %q", f.SuggestedSkill, "find-bugs")
	}
	gotText := source[f.Location.StartByte:f.Location.EndByte]
	if gotText != tt.actCallText {
		t.Errorf("Finding.Location text = %q, want %q (Location must point at the act call, not the check call)", gotText, tt.actCallText)
	}
}

type tsTOCTOUSourceCase struct {
	name   string
	source string
}

func expectNoTSTOCTOUFinding(t *testing.T, tt tsTOCTOUSourceCase) {
	root, closeTree := mustParseTS(t, []byte(tt.source))
	defer closeTree()

	_, findings := computeTSFeatures(root, []byte(tt.source))
	for _, f := range findings {
		if f.Kind == "toctou_check_then_act" {
			t.Fatalf("computeTSFeatures for %q: got toctou_check_then_act finding %+v, want none", tt.source, f)
		}
	}
}
