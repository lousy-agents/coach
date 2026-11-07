package tssetup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
)

// TestProjectPackageWorkingDirectoryUsesDiscoveredRootsWhenConfigPathEmpty
// proves an empty --project-config resolves the working directory from the
// discovered policy. Two canonical roots that each own a package.json are
// ambiguous; a missing project.json must not collapse them to the worktree root.
func TestProjectPackageWorkingDirectoryUsesDiscoveredRootsWhenConfigPathEmpty(t *testing.T) {
	repo := gitfixture.Init(t)
	for _, pkg := range []string{"packages/a", "packages/b"} {
		target := filepath.Join(repo, filepath.FromSlash(pkg), "package.json")
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", pkg, err)
		}
		if err := os.WriteFile(target, []byte("{\"name\":\"pkg\",\"version\":\"1.0.0\"}\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", target, err)
		}
	}
	revision := gitfixture.CommitFile(t, repo, ".lousy-agents/coach.json", `{"schema_version":"1","roots":["packages/a","packages/b"]}`+"\n")

	dir, reason := projectPackageWorkingDirectory(repo, revision, policyref.Ref{})
	if reason != setupChoiceReasonManifestContextAmbiguous || dir != "" {
		t.Fatalf("projectPackageWorkingDirectory with an empty project-config path = (%q, %q), want (\"\", %q)", dir, reason, setupChoiceReasonManifestContextAmbiguous)
	}
}

// An explicit policy that is committed and gone from disk is a policy gap, so
// the roots the readiness check used are none. Reading the committed copy
// instead would size the setup menu against a policy the scan refuses.
func TestCheckedPolicyRootsDoNotComeFromTheCommittedCopyOfAnExplicitPolicyDeletedOnDisk(t *testing.T) {
	repo := gitfixture.Init(t)
	revision := gitfixture.CommitFile(t, repo, "pol.json", `{"schema_version":"1","roots":["packages/a","packages/b"]}`+"\n")
	if err := os.Remove(filepath.Join(repo, "pol.json")); err != nil {
		t.Fatalf("remove pol.json: %v", err)
	}

	roots, err := checkedPolicyRoots(repo, revision, policyref.Named("pol.json"))

	if err != nil || len(roots) != 0 {
		t.Fatalf("checkedPolicyRoots = %v, %v; want no roots and no error for a policy the check failed", roots, err)
	}
}

func TestCheckedPolicyRootsStillComeFromTheCommittedCopyOfADiscoveredPolicy(t *testing.T) {
	repo := gitfixture.Init(t)
	revision := gitfixture.CommitFile(t, repo, ".lousy-agents/coach.json", `{"schema_version":"1","roots":["packages/a","packages/b"]}`+"\n")

	roots, err := checkedPolicyRoots(repo, revision, policyref.Ref{})

	if err != nil || len(roots) != 2 {
		t.Fatalf("checkedPolicyRoots = %v, %v; want the two committed roots", roots, err)
	}
}

func TestCheckedPolicyRootsStillComeFromTheCommittedCopyOfADiscoveredPolicyDeletedOnDisk(t *testing.T) {
	repo := gitfixture.Init(t)
	revision := gitfixture.CommitFile(t, repo, ".lousy-agents/coach.json", `{"schema_version":"1","roots":["packages/a","packages/b"]}`+"\n")
	if err := os.Remove(filepath.Join(repo, ".lousy-agents", "coach.json")); err != nil {
		t.Fatalf("remove coach.json: %v", err)
	}

	roots, err := checkedPolicyRoots(repo, revision, policyref.Ref{})

	if err != nil || len(roots) != 2 {
		t.Fatalf("checkedPolicyRoots = %v, %v; want the two committed roots, as before the explicit check existed", roots, err)
	}
}
