package projectconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAFileWhoseNameCannotBePrintedGetsTheReviewWithoutTheCommand(t *testing.T) {
	offer := workingCopyOffer(workingCopyChanged, "HEAD", "cfg/p\x1b.json")

	if strings.Contains(offer.text, "\x1b") || !strings.Contains(offer.text, "review it (the name has a character that is not printed here)") {
		t.Errorf("offer %q; want the step named and the unprintable name left out", offer.text)
	}
}

func TestGitIsAskedAboutTheFileOnlyWhereThereIsOne(t *testing.T) {
	repo := committedInvalidPolicy(t)
	if err := os.Remove(filepath.Join(repo, replacePath)); err != nil {
		t.Fatal(err)
	}

	changed, err := differsOnDisk(runProjectConfigGit, repo, "HEAD", replacePath)

	if err != nil || changed {
		t.Errorf("differsOnDisk = %v, %v; want a file that is gone to have no work in it", changed, err)
	}
}
