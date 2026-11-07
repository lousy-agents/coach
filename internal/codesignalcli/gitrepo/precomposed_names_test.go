package gitrepo

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// git on macOS precomposes the names it is given and the names it lists, so asking for a decomposed
// name gets the precomposed one back. That is the entry asked for, not another path's, and a probe
// that took it for another path's would refuse a policy because of how its name was typed.
func TestTheProbesTakeTheNameGitPrecomposesForTheNameAskedFor(t *testing.T) {
	t.Run("the index entry of the path", func(t *testing.T) {
		got, err := IndexEntries(answering(records(entryLine(precomposed))), "repo", decomposed)
		if err != nil || len(got.Entries) != 1 {
			t.Errorf("IndexEntries = %+v, %v; want the one entry git listed under the precomposed name", got, err)
		}
	})
	t.Run("the index entries below a directory", func(t *testing.T) {
		got, err := IndexEntries(answeringEach(nil, records("pé/inner.json")), "repo", "pé")
		if err != nil || !got.HoldsBelow {
			t.Errorf("IndexEntries = %+v, %v; want a directory", got, err)
		}
	})
	t.Run("the kind the revision holds", func(t *testing.T) {
		record := "100644 blob " + listingBlob + "\t" + precomposed + "\x00"
		got, err := EntryKindAtRevision(answering([]byte(record)), "repo", "HEAD", decomposed)
		if err != nil || got != EntryRegularFile {
			t.Errorf("EntryKindAtRevision = %v, %v; want a regular file", got, err)
		}
	})
}

// A record for a name that is not the one asked about still fails closed, and says so in Coach's own
// words: it is Coach's finding about what git printed, not something git said.
func TestARecordForAnotherNameIsCoachsOwnFindingAndNotGitsWords(t *testing.T) {
	_, err := IndexEntries(answering(records(entryLine("other.json"))), "repo", "pol.json")

	var operational *OperationalError
	var finding *UnexpectedOutputError
	if !errors.As(err, &operational) || !errors.As(operational.Cause, &finding) {
		t.Fatalf("IndexEntries error = %#v; want an *OperationalError caused by an *UnexpectedOutputError", err)
	}
}

// With a real git that precomposes (git on macOS does by default; others never do), a policy whose
// name was committed and is asked for in the other Unicode form is found in both forms.
func TestARealGitThatPrecomposesIsFollowedInBothForms(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	writeDecomposed(t, repo)
	gitIn(t, repo, "add", "-A")
	gitIn(t, repo, "-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-qm", "decomposed")
	if listed := gitIn(t, repo, "ls-files", "-z", "--", "cfg"); !bytes.Contains([]byte(listed), []byte(precomposed)) {
		t.Skip("this git does not precompose the names it lists")
	}

	for _, asked := range []string{precomposed, decomposed} {
		entries, err := IndexEntries(RunBytes, repo, asked)
		if err != nil || len(entries.Entries) != 1 {
			t.Errorf("IndexEntries for %q = %+v, %v; want the one entry", asked, entries, err)
		}
		kind, err := EntryKindAtRevision(RunBytes, repo, "HEAD", asked)
		if err != nil || kind != EntryRegularFile {
			t.Errorf("EntryKindAtRevision for %q = %v, %v; want a regular file", asked, kind, err)
		}
	}
}

func writeDecomposed(t *testing.T, repo string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(repo, "cfg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, decomposed), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitIn(t *testing.T, repo string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}
