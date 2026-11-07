package projectcheck

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func notARepository(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel").Run() == nil {
		t.Fatalf("%s is inside a git repository, so git can find a root there; point TMPDIR at a directory outside any repository", dir)
	}
	return dir
}

// A named policy is judged on disk. When git cannot find the repository the
// working tree cannot be checked at all, so the check must fail instead of
// passing the committed copy of a file nobody looked at.
func TestCheckExplicitPolicyFailsClosedWhenGitCannotFindTheRepository(t *testing.T) {
	check, roots, err := checkExplicitPolicy(readinessGit(), notARepository(t), "HEAD", "p.json")

	if err == nil {
		t.Fatalf("got no error and the check %+v; the policy would pass without the working tree ever being checked", check)
	}
	if check.State == projectreadiness.Pass {
		t.Errorf("check state = %q alongside the error %q, want no pass", check.State, err)
	}
	if roots != nil {
		t.Errorf("roots = %v, want none from a policy that was never read", roots)
	}
	var verdict *projectconfig.ConfigError
	if errors.As(err, &verdict) {
		t.Errorf("error %q is a *ConfigError, a verdict about the policy; it must be an operational failure", err)
	}
}
