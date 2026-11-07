package gitrepo

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// A snapshot of a partial clone that lacks several objects says how many, and which path comes first, so the
// user knows the size of what has to be fetched and one place to start.
func TestAnUnavailableRevisionNamesHowManyObjectsAndTheFirstPath(t *testing.T) {
	skipIfGitCannotRefuseLazyFetch(t)
	source := gitfixture.Init(t)
	for _, name := range []string{"a.txt", "b.txt", "c.txt"} {
		gitfixture.CommitFile(t, source, name, "content of "+name)
	}
	bare := filepath.Join(t.TempDir(), "bare.git")
	gitOutput(t, source, "clone", "--bare", "--quiet", source, bare)
	gitOutput(t, bare, "config", "uploadpack.allowFilter", "true")
	clone := filepath.Join(t.TempDir(), "part")
	gitOutput(t, t.TempDir(), "clone", "--quiet", "--no-checkout", "--filter=blob:none", "file://"+bare, clone)

	_, err := ExtractRevision(clone, "HEAD")

	var absent *ObjectsNotLocalError
	if !errors.As(err, &absent) || absent.Count != 3 || absent.FirstPath != "a.txt" {
		t.Errorf("ExtractRevision = %v; want an ObjectsNotLocalError for 3 objects whose first path is a.txt", err)
	}
}
