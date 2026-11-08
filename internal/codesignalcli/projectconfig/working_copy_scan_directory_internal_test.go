package projectconfig

import (
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// The conversion is asked of git from the repository root, which the policy's path is relative to, wherever the scan
// started: a clean checkout that git converted is the committed copy from a subdirectory as it is from the root. The git
// is real.
func TestAConvertedCheckoutIsNotChangedWhenTheScanStartsInASubdirectory(t *testing.T) {
	repo, objectID := aConvertedCheckout(t, "core.autocrlf", "true")

	t.Run("the root", func(t *testing.T) { requireUnchangedWhenScannedFrom(t, repo, objectID) })
	t.Run("a subdirectory", func(t *testing.T) { requireUnchangedWhenScannedFrom(t, filepath.Join(repo, "cfg"), objectID) })
}

func requireUnchangedWhenScannedFrom(t *testing.T, dir, objectID string) {
	t.Helper()
	changed, err := differsOnDisk(NewGitRunner(), dir, gitrepo.TreeEntry{Mode: "100644", SHA: objectID}, true, "cfg/p.json")

	if err != nil || changed {
		t.Errorf("differsOnDisk from %s = %v, %v; want false, nil", dir, changed, err)
	}
}
