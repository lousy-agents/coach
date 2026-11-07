package main

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// The refusal of compiler setup says what the scan says about a committed policy whose index holds only
// an empty entry for it: the restore replaces that entry and loses nothing. It builds the sentence with
// the scan's own builder, so the two cannot drift.
func TestPolicyRequiredLineExplainsARestoreOverAnEmptyIndexEntryAsTheScanDoes(t *testing.T) {
	line := policyRequiredLine("coach codesignal", "policy_missing", projectreadiness.RemedyRestoreOverEmptyEntry, "abc123", "pol.json")

	want := projectconfig.RestoreSentence(projectreadiness.RemedyRestoreOverEmptyEntry, "abc123", "pol.json", projectconfig.AsARefusal(projectreadiness.NextActionRestorePolicy))
	if !strings.HasSuffix(line, want) || !strings.Contains(line, "empty entry") || !strings.Contains(line, "restore_policy") {
		t.Errorf("line %q does not end with the shared sentence %q", line, want)
	}
}
