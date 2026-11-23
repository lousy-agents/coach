package projectcheck

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
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

// A policy is judged on disk. When git cannot find the repository the
// working tree cannot be checked at all, so the check must fail instead of
// passing a file nobody looked at.
func TestJudgePolicyFailsClosedWhenGitCannotFindTheRepository(t *testing.T) {
	judgment, err := judgePolicy(readinessGit(), notARepository(t), "HEAD", policyref.Named("p.json"))
	check, roots := judgment.check, judgment.roots

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
