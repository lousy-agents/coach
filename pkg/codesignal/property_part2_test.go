package codesignal

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/semantics"
)

func TestProperty_ArbitraryEvidenceProducesValidJSON(t *testing.T) {
	longEvidence := ""
	for i := 0; i < 10000; i++ {
		longEvidence += "x"
	}

	tests := []struct {
		name     string
		evidence string
	}{
		{"embedded double quotes", `cfg.Name = "hello \"world\""`},
		{"backslashes", `path = "C:\\Users\\name"`},
		{"multi-byte unicode", "变量.名前 =値"},
		{"emoji", "cfg.Emoji = \"🎉🚀💥\""},
		{"tab and newline control characters", "x = 1\t// comment\ny = 2"},
		{"embedded null byte", "x\x00 = 1"},
		{"very long string", longEvidence},
		{"mixed adversarial", "x = \"\\n\\t\x00\" + 变量 + \"🎉\""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body_propertyPart2Test_33(t, tt)
		})
	}
}

func reverseFileChanges(files []FileChange) []FileChange {
	out := make([]FileChange, len(files))
	for i, fc := range files {
		reordered := fc
		if fc.Base != nil {
			baseCopy := *fc.Base
			baseCopy.Findings = reverseFindings(fc.Base.Findings)
			reordered.Base = &baseCopy
		}
		if fc.Head != nil {
			headCopy := *fc.Head
			headCopy.Findings = reverseFindings(fc.Head.Findings)
			reordered.Head = &headCopy
		}
		out[len(files)-1-i] = reordered
	}
	return out
}

func reverseFindings(findings []semantics.Finding) []semantics.Finding {
	if findings == nil {
		return nil
	}
	out := make([]semantics.Finding, len(findings))
	for i, f := range findings {
		out[len(findings)-1-i] = f
	}
	return out
}
