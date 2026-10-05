package codesignalcli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// TestCheckProjectShapeIgnoresRootsWhenPolicyNotPassed pins the
// policyPassed guard in checkProjectShape directly, independent of whatever
// checkPolicy happens to return on any particular invalid-policy path:
// removing the `if !policyPassed` branch turns this red regardless. Without
// a validated policy, roots is untrusted input, so a root-level manifest
// miss reports not_checked rather than asserting an unsupported shape this
// check has no basis to claim (R1) -- a genuine monorepo whose manifests
// live under an as-yet-uncommitted root must not be misreported as
// projectreadiness.GapUnsupportedRepositoryShape purely because its policy is missing.
func TestCheckProjectShapeIgnoresRootsWhenPolicyNotPassed(t *testing.T) {
	repo := gitfixture.Init(t)
	if err := os.MkdirAll(filepath.Join(repo, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	revision := gitfixture.CommitFile(t, repo, "sub/package.json", `{"name":"example","version":"1.0.0"}`+"\n")

	got, err := checkProjectShape(repo, revision, []string{"sub"}, false)
	if err != nil {
		t.Fatalf("checkProjectShape returned error: %v", err)
	}
	if got.State != projectreadiness.NotChecked {
		t.Fatalf("State = %q, want %q", got.State, projectreadiness.NotChecked)
	}
	if got.Code != "" {
		t.Fatalf("Code = %q, want empty: not_checked carries no gap code", got.Code)
	}
}
