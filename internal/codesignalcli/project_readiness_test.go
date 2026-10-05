package codesignalcli

import (
	"os"
	"os/exec"
	"path/filepath"

	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func TestFileExistsAtRevisionIgnoresTreeEntries(t *testing.T) {
	repo := gitfixture.Init(t)
	if err := os.Mkdir(filepath.Join(repo, "package.json"), 0o755); err != nil {
		t.Fatalf("mkdir package.json: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "package.json", "inner"), []byte("not a blob\n"), 0o644); err != nil {
		t.Fatalf("write inner: %v", err)
	}

	addCmd := exec.Command("git", "add", "package.json")
	addCmd.Dir = repo
	if output, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v: %s", err, output)
	}
	commitCmd := exec.Command("git", "commit", "-m", "tree named package.json")
	commitCmd.Dir = repo
	commitCmd.Env = gitfixture.CommitEnv
	if output, err := commitCmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit: %v: %s", err, output)
	}
	revCmd := exec.Command("git", "rev-parse", "HEAD")
	revCmd.Dir = repo
	revOut, err := revCmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse HEAD: %v", err)
	}
	revision := strings.TrimSpace(string(revOut))

	exists, err := fileExistsAtRevision(runProjectConfigGit, repo, revision, "package.json")
	if err != nil {
		t.Fatalf("fileExistsAtRevision returned error: %v", err)
	}
	if exists {
		t.Fatal("a tree named package.json must not count as the package.json blob")
	}
}

// TestCheckProjectShapeIgnoresRootsWhenPolicyNotPassed pins the
// policyPassed guard in checkProjectShape directly, independent of whatever
// checkPolicy happens to return on any particular invalid-policy path:
// removing the `if !policyPassed` branch turns this red regardless. Without
// a validated policy, roots is untrusted input, so a root-level manifest
// miss reports not_checked rather than asserting an unsupported shape this
// check has no basis to claim (R1) -- a genuine monorepo whose manifests
// live under an as-yet-uncommitted root must not be misreported as
// GapUnsupportedRepositoryShape purely because its policy is missing.
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
	if got.State != ReadinessNotChecked {
		t.Fatalf("State = %q, want %q", got.State, ReadinessNotChecked)
	}
	if got.Code != "" {
		t.Fatalf("Code = %q, want empty: not_checked carries no gap code", got.Code)
	}
}
