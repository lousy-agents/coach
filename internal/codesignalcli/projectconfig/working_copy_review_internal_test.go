package projectconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

func TestAFileWhoseNameCannotBePrintedGetsTheReviewWithoutTheCommand(t *testing.T) {
	offer := workingCopyOffer(workingCopyChanged, "HEAD", "cfg/p\x1b.json")

	if strings.Contains(offer.text, "\x1b") || !strings.Contains(offer.text, "review it (the name has a character that is not printed here)") {
		t.Errorf("offer %q; want the step named and the unprintable name left out", offer.text)
	}
}

func TestAFileThatIsGoneHasNoWorkInIt(t *testing.T) {
	repo := committedInvalidPolicy(t)
	if err := os.Remove(filepath.Join(repo, replacePath)); err != nil {
		t.Fatal(err)
	}

	changed, err := differsOnDisk(runProjectConfigGit, repo, gitrepo.TreeEntry{Mode: "100644", SHA: strings.Repeat("0", 40)}, true, replacePath)

	if err != nil || changed {
		t.Errorf("differsOnDisk = %v, %v; want a file that is gone to have no work in it", changed, err)
	}
}

// What the committed entry is decides nothing about a file that is there when it cannot be told: no entry found, or one
// that is not a regular blob, is taken to have work in the file, however alike the file and the object are.
func TestAFileThatIsThereHasWorkInItWhenTheCommittedEntryCannotBeCompared(t *testing.T) {
	repo, objectID := aCommittedBlob(t)
	cases := map[string]struct {
		entry gitrepo.TreeEntry
		found bool
		want  bool
	}{
		"the entry is the file as committed": {gitrepo.TreeEntry{Mode: "100644", SHA: objectID}, true, false},
		"the entry is an executable file":    {gitrepo.TreeEntry{Mode: "100755", SHA: objectID}, true, false},
		"no entry is committed":              {gitrepo.TreeEntry{Mode: "100644", SHA: objectID}, false, true},
		"the entry is a link":                {gitrepo.TreeEntry{Mode: "120000", SHA: objectID}, true, true},
		"the entry is another object":        {gitrepo.TreeEntry{Mode: "100644", SHA: strings.Repeat("1", 40)}, true, true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireDiffersOnDisk(t, repo, c.entry, c.found, c.want) })
	}
}

func requireDiffersOnDisk(t *testing.T, repo string, entry gitrepo.TreeEntry, found, want bool) {
	t.Helper()
	changed, err := differsOnDisk(runProjectConfigGit, repo, entry, found, "cfg/p.json")

	if err != nil || changed != want {
		t.Errorf("differsOnDisk = %v, %v; want %v", changed, err, want)
	}
}
