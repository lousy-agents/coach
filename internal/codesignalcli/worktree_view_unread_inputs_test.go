package codesignalcli

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestHasOwnDiagnosticIsTrueOnlyForTheSelectedPolicy(t *testing.T) {
	inputs := unreadInputs{policy: "config/policy.json"}

	for name, want := range map[string]bool{
		"config/policy.json":       true,
		"project.json":             false,
		".lousy-agents/coach.json": false,
		"pkg/main.go":              false,
	} {
		if got := inputs.hasOwnDiagnostic(name); got != want {
			t.Errorf("hasOwnDiagnostic(%q) = %v, want %v: only the policy the scan selected is read from disk", name, got, want)
		}
	}
}

func TestHasOwnDiagnosticTreatsAnNFCVariantSpellingOfTheSelectedPolicyAsTheSameFile(t *testing.T) {
	inputs := unreadInputs{policy: "caf\u00e9/policy.json"}

	if !inputs.hasOwnDiagnostic("cafe\u0301/policy.json") {
		t.Error("an NFD spelling of the selected policy path was not recognised as the selected policy")
	}
}

func TestAPolicyFileTheSelectionDidNotPickIsDisclosedAsUnreadWhateverItsStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		porcelain  []string
		policyPath string
		want       string
	}{
		{"untracked", []string{"?? project.json"}, ".lousy-agents/coach.json", "project.json"},
		{"staged new", []string{"A  project.json"}, ".lousy-agents/coach.json", "project.json"},
		{"intent-to-add", []string{" A project.json"}, ".lousy-agents/coach.json", "project.json"},
		{"renamed in", []string{"R  project.json\x00draft.json"}, ".lousy-agents/coach.json", "project.json"},
		{"edited beside the selected canonical policy", []string{" M project.json"}, ".lousy-agents/coach.json", "project.json"},
		{"canonical edited beside the selected deprecated policy", []string{" M .lousy-agents/coach.json"}, "project.json", ".lousy-agents/coach.json"},
		{"canonical staged beside the selected deprecated policy", []string{"A  .lousy-agents/coach.json"}, "project.json", ".lousy-agents/coach.json"},
		{"edited beside a policy named with --project-config", []string{" M project.json"}, "config/policy.json", "project.json"},
		{"the selected policy edited", []string{" M project.json"}, "project.json", ""},
		{"the selected policy untracked", []string{"?? .lousy-agents/coach.json"}, ".lousy-agents/coach.json", ""},
		{"a policy-named file in a subdirectory is not a policy file but is disclosed like any other", []string{"A  sub/project.json"}, ".lousy-agents/coach.json", "sub/project.json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertUnreadDisclosure(t, tt.porcelain, tt.policyPath, tt.want)
		})
	}
}

// assertUnreadDisclosure states the disclosure as the single informational line
// that lists the unread paths, or as no disclosure at all.
func assertUnreadDisclosure(t *testing.T, porcelain []string, policyPath, want string) {
	t.Helper()
	entries := statusFromPorcelain(porcelainZ(porcelain...), nil).Entries
	inputs := (&WorktreeView{}).unreadInputs(entries, policyPath)
	var unread []gitrepo.WorktreeEntry
	for _, entry := range entries {
		if !inputs.hasOwnDiagnostic(entry.Path) {
			unread = append(unread, entry)
		}
	}

	diagnostics := perPathDisclosure(collectWorktreePaths(unread), uniqueSorted(inputs.config))

	if want == "" {
		if len(diagnostics) != 0 {
			t.Fatalf("disclosed %v, want nothing", diagnostics)
		}
		return
	}
	if len(diagnostics) != 1 || diagnostics[0].Kind != codesignal.DiagKindWorktreeNotClean {
		t.Fatalf("disclosed %v, want one %s line", diagnostics, codesignal.DiagKindWorktreeNotClean)
	}
	if got := diagnostics[0].Message; !strings.HasSuffix(got, ": "+want) || strings.Contains(got, "commit") {
		t.Errorf("message %q, want it to list %s as unread without a claim about the commit", got, want)
	}
}
