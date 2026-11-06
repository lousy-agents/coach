package gitrepo

import (
	"os"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func TestMissingObjectsOfCountsTheObjectsAPartialCloneLacks(t *testing.T) {
	clone, _ := partialClone(t, func(string) string { return "ssh://example.invalid/repo.git" })
	ordinary := gitfixture.Init(t)
	gitfixture.CommitFile(t, ordinary, "cfg/p.json", "{}")

	lacking, err := missingObjectsOf(RunBytes, clone, "HEAD")
	whole, wholeErr := missingObjectsOf(RunBytes, ordinary, "HEAD")

	if err != nil || lacking != (missingObjects{count: 1}) {
		t.Errorf("missingObjectsOf(partial clone) = %+v, %v; want the one blob the clone lacks", lacking, err)
	}
	if wholeErr != nil || whole.any() {
		t.Errorf("missingObjectsOf(ordinary repository) = %+v, %v; want nothing missing", whole, wholeErr)
	}
}

// The probe is what lets an older git read a partial clone, so it must not be the read that fetches. The
// runner here leaves out GIT_NO_LAZY_FETCH, as an older git would ignore it, and a control read shows that
// the setup would notice a fetch: git runs the ssh program the moment a blob it lacks is asked for.
func TestTheProbeFetchesNothingEvenWhenGitIsNotToldNotTo(t *testing.T) {
	marker := sshMarker(t)
	clone, blob := partialClone(t, func(string) string { return "ssh://example.invalid/repo.git" })
	before := packFiles(t, clone)
	run := gitThatMayFetch()

	_, _ = run(clone, "cat-file", "-p", blob)
	if _, err := os.Stat(marker); err != nil {
		t.Skipf("this git does not fetch a blob it lacks when asked for it, so the control read cannot show the probe is safe: %v", err)
	}
	if err := os.Remove(marker); err != nil {
		t.Fatalf("clear the marker: %v", err)
	}

	lacking, err := missingObjectsOf(run, clone, "HEAD")

	if err != nil || lacking.count != 1 {
		t.Fatalf("missingObjectsOf = %+v, %v; want the one blob the clone lacks", lacking, err)
	}
	if _, err := os.Stat(marker); err == nil {
		t.Error("the probe ran the ssh program: it fetched where it should only have listed")
	}
	if after := packFiles(t, clone); after != before {
		t.Errorf("the probe took the object store from %d to %d pack files", before, after)
	}
}

func gitThatMayFetch() Runner {
	return func(dir string, args ...string) ([]byte, error) {
		command := exec.Command("git", args...)
		command.Dir = dir
		command.Env = slices.DeleteFunc(os.Environ(), func(entry string) bool { return strings.HasPrefix(entry, "GIT_NO_LAZY_FETCH=") })
		return command.Output()
	}
}

func TestRevisionsReadAreHEADAndTheMergeBaseOfTheBase(t *testing.T) {
	repository := gitfixture.Init(t)
	fork := gitfixture.CommitFile(t, repository, "a.go", "package a\n")
	trunk := strings.TrimSpace(gitfixture.Git(t, repository, "branch", "--show-current"))
	gitfixture.Git(t, repository, "branch", "side")
	gitfixture.CommitFile(t, repository, "b.go", "package a\n")
	gitfixture.Git(t, repository, "checkout", "--quiet", "side")
	gitfixture.CommitFile(t, repository, "c.go", "package a\n")

	baseline, baselineErr := revisionsRead(RunBytes, repository, "")
	diff, diffErr := revisionsRead(RunBytes, repository, trunk)

	if baselineErr != nil || len(baseline) != 1 || baseline[0].name != "HEAD" {
		t.Errorf("revisionsRead(baseline) = %+v, %v; want HEAD alone", baseline, baselineErr)
	}
	if diffErr != nil || len(diff) != 2 || diff[0].name != "HEAD" || diff[1].name != fork {
		t.Errorf("revisionsRead(diff) = %+v, %v; want HEAD and the fork point %s", diff, diffErr, fork)
	}
}
