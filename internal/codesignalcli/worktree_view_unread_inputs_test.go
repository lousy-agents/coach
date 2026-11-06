package codesignalcli

import (
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestUnreadInputsNamesAPolicyFileWhateverItsStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		porcelain  []string
		policyPath string
		want       []string
	}{
		{"untracked", []string{"?? project.json"}, "", []string{"project.json: new"}},
		{"staged new", []string{"A  project.json"}, "", []string{"project.json: new"}},
		{"intent-to-add", []string{" A project.json"}, "", []string{"project.json: new"}},
		{"renamed in", []string{"R  project.json\x00draft.json"}, "", []string{"project.json: new"}},
		{"canonical staged beside a selected deprecated policy", []string{"A  .lousy-agents/coach.json"}, "project.json", []string{".lousy-agents/coach.json: new"}},
		{"a committed policy the selection did not pick, edited", []string{" M project.json"}, "", []string{"project.json: changed"}},
		{"a policy the selection did not pick, renamed away: the old name is a change, not a new file", []string{"R  moved.json\x00project.json"}, "", []string{"project.json: changed"}},
		{"the selected policy edited", []string{" M project.json"}, "project.json", []string{"project.json: selected"}},
		{"the selected policy renamed away", []string{"R  moved.json\x00project.json"}, "project.json", []string{"project.json: selected"}},
		{"the selected policy renamed to the canonical path", []string{"R  .lousy-agents/coach.json\x00project.json"}, "project.json", []string{"project.json: selected", ".lousy-agents/coach.json: new"}},
		{"a policy-named file in a subdirectory is not a policy", []string{"A  sub/project.json"}, "", nil},
		{"another file is not a policy", []string{"A  draft.json"}, "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertPolicyDiagnostics(t, tt.porcelain, tt.policyPath, tt.want)
		})
	}
}

// assertPolicyDiagnostics states each diagnostic as its path and which message
// it carries: the selected policy's, a file HEAD does not hold, or an uncommitted
// change to one it holds.
func assertPolicyDiagnostics(t *testing.T, porcelain []string, policyPath string, want []string) {
	t.Helper()
	view := newWorktreeViewForTest(t, worktreeViewDeps{
		status:    statusFromPorcelain(porcelainZ(porcelain...), nil),
		committed: fstest.MapFS{},
		disk:      absentDisk{},
	})

	inputs := view.unreadInputs(view.status.Entries, policyPath)

	var got []string
	for _, diagnostic := range inputs.policyDiagnostics("rev") {
		got = append(got, diagnostic.Path+": "+policyMessageKind(diagnostic.Message))
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("policy diagnostics %v, want %v", got, want)
	}
}

func policyMessageKind(message string) string {
	switch {
	case strings.Contains(message, "is not committed"):
		return "new"
	case strings.Contains(message, "has uncommitted changes"):
		return "changed"
	}
	return "selected"
}

func TestHasOwnDiagnosticCoversTheSelectedPolicyAndEveryOtherPolicyFileNamed(t *testing.T) {
	inputs := unreadInputs{policy: "a.json", others: []policyChange{{path: "b.json"}}}

	for name, want := range map[string]bool{"a.json": true, "b.json": true, "c.json": false} {
		if got := inputs.hasOwnDiagnostic(name); got != want {
			t.Errorf("hasOwnDiagnostic(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestPolicyChangeSaysWhetherTheFileIsNotCommittedOrHasUncommittedChanges(t *testing.T) {
	tests := []struct {
		name   string
		change policyChange
		want   string
	}{
		{"a file HEAD does not hold", policyChange{path: "project.json", new: true},
			"the project policy at project.json is not committed; --worktree reads the policy from the commit at rev and does not read an uncommitted policy yet"},
		{"a file HEAD holds", policyChange{path: ".lousy-agents/coach.json"},
			"the project policy file at .lousy-agents/coach.json has uncommitted changes; --worktree reads the policy from the commit at rev and does not read an uncommitted policy yet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertPolicyChangeMessage(t, tt.change, tt.want)
		})
	}
}

func assertPolicyChangeMessage(t *testing.T, change policyChange, want string) {
	t.Helper()
	if got := change.message("rev"); got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}
