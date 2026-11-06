package codesignalcli

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func TestUnreadConfigIgnoresDirectoryMarkersAndDependencies(t *testing.T) {
	t.Run("a config file the scan did not read", func(t *testing.T) { assertUnreadConfig(t, "pkg/go.mod", true) })
	t.Run("an untracked directory that happens to be named like a config file", func(t *testing.T) { assertUnreadConfig(t, "pkg/go.mod/", false) })
	t.Run("a config file under node_modules", func(t *testing.T) { assertUnreadConfig(t, "node_modules/pkg/go.mod", false) })
	t.Run("a file that is not a config input", func(t *testing.T) { assertUnreadConfig(t, "pkg/main.go", false) })
}

func assertUnreadConfig(t *testing.T, path string, want bool) {
	t.Helper()
	if got := (&WorktreeView{}).unreadConfig(path); got != want {
		t.Errorf("unreadConfig(%q) = %v, want %v", path, got, want)
	}
}

func TestAPolicyFileNamedByTwoEntriesIsDisclosedOnce(t *testing.T) {
	entries := []gitrepo.WorktreeEntry{
		{Code: " D", Path: projectconfig.DefaultPath},
		{Code: "R ", Path: projectconfig.CanonicalPath, OldPath: projectconfig.DefaultPath},
	}

	inputs := (&WorktreeView{}).unreadInputs(entries, "elsewhere/policy.json")
	got := disclosedPaths(inputs.policyDiagnostics("abc123"), codesignal.DiagKindWorktreeChangesNotAnalyzed)

	if want := []string{projectconfig.CanonicalPath, projectconfig.DefaultPath}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("policy diagnostics name %v, want %v: each policy file once, in path order", got, want)
	}
}

func TestUnreadChangesMessageMarksATruncatedSampleAndOnlyThat(t *testing.T) {
	few := []string{"a.txt", "b.txt"}
	many := []string{"a.txt", "b.txt", "c.txt", "d.txt", "e.txt", "f.txt", "g.txt"}

	if got := unreadChangesDiagnostic(few)[0].Message; strings.Contains(got, "…") {
		t.Errorf("a complete sample was marked as cut short: %q", got)
	}
	if got := unreadChangesDiagnostic(many)[0].Message; !strings.HasSuffix(got, ", …") || strings.Contains(got, "f.txt") {
		t.Errorf("a sample cut at %d paths rendered %q, want the first %d and a trailing ellipsis", worktreeDisclosureSampleLimit, got, worktreeDisclosureSampleLimit)
	}
}
