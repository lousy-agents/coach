package projectconfig

import (
	"errors"
	"slices"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

var rootLookup = []string{"rev-parse", "--show-toplevel"}

// The repository root is a git read like the others, so it goes through the runner
// the caller holds to its limits. A lookup that built its own git command ran
// without the output caps and the wall-time limit every other read of the policy has.
func TestTheRepositoryRootIsFoundThroughTheRunnerTheCallerInjected(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	lookups := 0
	run := func(dir string, args ...string) ([]byte, error) {
		if slices.Equal(args, rootLookup) {
			lookups++
		}
		return gitrepo.RunBytes(dir, args...)
	}

	status, err := ExplicitOnDiskStatus(run, repo, "HEAD", "cfg/p.json")

	if err != nil || status == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want a missing-policy error", status, err)
	}
	if lookups == 0 {
		t.Error("the repository root was never asked of the injected runner: the lookup ran outside the limits the caller set")
	}
}

func TestARunnerThatCannotFindTheRepositoryRootFailsTheCheckInsteadOfJudgingAPolicy(t *testing.T) {
	repo := committedRepository(t)
	boom := errors.New("root lookup exceeded its limit")

	status, err := ExplicitOnDiskStatus(indexFake{rootErr: boom}.run, repo, "HEAD", "cfg/p.json")

	if status != nil || !errors.Is(err, boom) {
		t.Errorf("ExplicitOnDiskStatus = %+v, %v; want no verdict and the runner's own failure", status, err)
	}
}

// recordRootLookups replaces the seam Load reads with and counts how often the
// repository root is asked of it.
func recordRootLookups(t *testing.T) *int {
	t.Helper()
	lookups := new(int)
	original := runProjectConfigGit
	t.Cleanup(func() { runProjectConfigGit = original })
	runProjectConfigGit = func(dir string, args ...string) ([]byte, error) {
		if slices.Equal(args, rootLookup) {
			*lookups++
		}
		return original(dir, args...)
	}
	return lookups
}

func committedRepository(t *testing.T) string {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	return repo
}

// Discover and the not-found message look the root up too. They go through the
// seam Load reads with, which a test replaces to see what it is asked.
func TestDiscoverFindsTheRepositoryRootThroughTheLoadSeam(t *testing.T) {
	repo := committedRepository(t)
	gitfixture.CommitFile(t, repo, CanonicalPath, `{"schema_version":"1","roots":["."]}`)
	lookups := recordRootLookups(t)

	_, _ = Discover(repo, "HEAD")

	if *lookups == 0 {
		t.Error("the repository root was never asked of the seam: the lookup ran outside the limits the seam sets")
	}
}

func TestTheNotFoundMessageFindsTheRepositoryRootThroughTheLoadSeam(t *testing.T) {
	repo := committedRepository(t)
	lookups := recordRootLookups(t)

	_, _ = Load(repo, "HEAD", "absent.json")

	if *lookups == 0 {
		t.Error("the repository root was never asked of the seam: the lookup ran outside the limits the seam sets")
	}
}
