package tssetup

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
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

	dir, reason := projectPackageWorkingDirectory(repo, revision, "")
	if reason != setupChoiceReasonManifestContextAmbiguous || dir != "" {
		t.Fatalf("projectPackageWorkingDirectory with an empty project-config path = (%q, %q), want (\"\", %q)", dir, reason, setupChoiceReasonManifestContextAmbiguous)
	}
}
