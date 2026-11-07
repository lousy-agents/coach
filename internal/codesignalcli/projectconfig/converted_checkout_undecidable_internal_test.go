package projectconfig

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// What git cannot answer is taken to differ, because a replacement offered over a file that was not understood can
// destroy what is in it.
func TestWhatGitCannotAnswerAboutAConvertedCheckoutIsTakenToDiffer(t *testing.T) {
	repo, objectID := aConvertedCheckout(t, "core.autocrlf", "true")
	cases := map[string]gitrepo.Runner{
		"the attribute question fails":  fakeGitAnswering(t, errors.New("exit status 128"), "", "unused"),
		"the attribute record is short": fakeGitAnswering(t, nil, "cfg/p.json\x00", objectID),
		"the hash question fails":       fakeGitAnswering(t, nil, "cfg/p.json\x00filter\x00unspecified\x00", ""),
		"the hash is another object":    fakeGitAnswering(t, nil, "cfg/p.json\x00filter\x00unspecified\x00", strings.Repeat("1", 40)),
	}
	for name, run := range cases {
		t.Run(name, func(t *testing.T) { requireTakenToDiffer(t, run, repo, objectID) })
	}
}

func requireTakenToDiffer(t *testing.T, run gitrepo.Runner, repo, objectID string) {
	t.Helper()
	if holdsTheCommittedBlobConverted(run, repo, "cfg/p.json", objectID) {
		t.Errorf("the file was taken for the committed blob")
	}
}

// fakeGitAnswering is a git that answers check-attr with attribute (or fails it with attributeErr), and hash-object with
// hash (an empty hash is a failure). Any other question is an error that names it.
func fakeGitAnswering(t *testing.T, attributeErr error, attribute, hash string) gitrepo.Runner {
	t.Helper()
	return func(_ string, args ...string) ([]byte, error) {
		switch args[0] {
		case "check-attr":
			return []byte(attribute), attributeErr
		case "hash-object":
			if hash == "" {
				return nil, errors.New("exit status 128: fatal: hash-object failed")
			}
			return []byte(hash + "\n"), nil
		}
		return nil, errors.New("unexpected git question: " + strings.Join(args, " "))
	}
}

func TestALinkIsNeverConverted(t *testing.T) {
	repo, objectID := aConvertedCheckout(t, "core.autocrlf", "true")
	elsewhere := filepath.Join(repo, "elsewhere.json")
	if err := os.WriteFile(elsewhere, []byte(committedBytes), 0o644); err != nil {
		t.Fatal(err)
	}
	replaceWithLink(t, filepath.Join(repo, "cfg", "p.json"), elsewhere)

	requireConverted(t, repo, objectID, false)
}

func TestTheConversionIsAskedOfGitFromTheRootAndWithTheFilesOwnPath(t *testing.T) {
	repo, objectID := aConvertedCheckout(t, "core.autocrlf", "true")
	var asked []string
	run := func(dir string, args ...string) ([]byte, error) {
		asked = append(asked, dir+": "+strings.Join(args, " "))
		return exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	}

	holdsTheCommittedBlobConverted(run, repo, "cfg/p.json", objectID)

	want := []string{repo + ": check-attr -z filter -- cfg/p.json", repo + ": hash-object --path=cfg/p.json -- " + filepath.Join(repo, "cfg", "p.json")}
	if strings.Join(asked, "\n") != strings.Join(want, "\n") {
		t.Errorf("asked %q; want %q", asked, want)
	}
}
