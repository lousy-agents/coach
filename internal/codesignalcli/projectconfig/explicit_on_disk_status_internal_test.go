package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// listingFails is real git with the index listing broken, so the working tree and
// the commit answer truthfully and only the index question fails.
func listingFails(dir string, args ...string) ([]byte, error) {
	if args[0] == "ls-files" {
		return nil, errors.New("injected failure")
	}
	return gitrepo.RunBytes(dir, args...)
}

func TestExplicitOnDiskStatusKeepsTheIndexUnreadWhenGitCannotListIt(t *testing.T) {
	cases := []struct {
		name  string
		hide  func(t *testing.T, repo string)
		cause MissingCause
	}{
		{"the file is gone", func(t *testing.T, repo string) { moveAside(t, repo, "cfg/p.json") }, CauseAbsent},
		{"a file stands where the parent directory was", func(t *testing.T, repo string) {
			moveAside(t, repo, "cfg")
			writeFile(t, repo, "cfg", "notes")
		}, CauseBlockedParent},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertIndexUnreadWhenListingFails(t, c.hide, c.cause)
		})
	}
}

func assertIndexUnreadWhenListingFails(t *testing.T, hide func(t *testing.T, repo string), cause MissingCause) {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
	hide(t, repo)

	status, err := ExplicitOnDiskStatus(listingFails, repo, "HEAD", "cfg/p.json")

	if err != nil || status == nil || status.Cause != cause {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want a missing policy with cause %d", status, err, cause)
	}
	if !status.UnreadStagedEdit {
		t.Errorf("UnreadStagedEdit is false; want true, because a report that cannot rule out a staged edit must not claim the working tree is all it read")
	}
	if cause == CauseBlockedParent && strings.Contains(status.Message, "could not check") {
		t.Errorf("message %q; want the blocked-parent message with no undecidable text", status.Message)
	}
}

// moveAside puts a path out of the way inside the repository's own temporary
// directory tree, so a test never deletes anything.
func moveAside(t *testing.T, repo, name string) {
	t.Helper()
	aside := filepath.Join(t.TempDir(), "aside")
	if err := os.Rename(filepath.Join(repo, name), aside); err != nil {
		t.Fatalf("move %s aside: %v", name, err)
	}
}
