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
	counting := &rootCounter{}

	status, err := ExplicitOnDiskStatus(counting.run, repo, "HEAD", "cfg/p.json")

	if err != nil || status == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want a missing-policy error", status, err)
	}
	counting.requireTheRootWasAsked(t)
}

// rootCounter is a runner handed to the code under test, as a caller hands its own: it counts the questions for the repository
// root and answers every question with real git. It is passed in; no package variable is replaced.
type rootCounter struct {
	lookups int
}

func (c *rootCounter) run(dir string, args ...string) ([]byte, error) {
	if slices.Equal(args, rootLookup) {
		c.lookups++
	}
	return gitrepo.RunBytes(dir, args...)
}

func (c *rootCounter) requireTheRootWasAsked(t *testing.T) {
	t.Helper()
	if c.lookups == 0 {
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

func committedRepository(t *testing.T) string {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	return repo
}

// Discover and the not-found message look the root up too. They ask the runner they are given, which Discover and Load take from
// the package's own seam, so the limits of the seam apply to the lookup as they do to every other read.
func TestDiscoverFindsTheRepositoryRootThroughTheRunnerItIsGiven(t *testing.T) {
	repo := committedRepository(t)
	gitfixture.CommitFile(t, repo, CanonicalPath, `{"schema_version":"1","roots":["."]}`)
	counting := &rootCounter{}

	_, _ = discoverThrough(counting.run, repo, "HEAD")

	counting.requireTheRootWasAsked(t)
}

func TestTheNotFoundMessageFindsTheRepositoryRootThroughTheRunnerItIsGiven(t *testing.T) {
	repo := committedRepository(t)
	counting := &rootCounter{}

	_, _ = loadThrough(counting.run, repo, "HEAD", "absent.json")

	counting.requireTheRootWasAsked(t)
}
