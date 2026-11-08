package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// project.json is the policy and nothing is on disk at the default path, but the index holds a staged copy of it. A
// move there would replace what is staged, so the path is taken, with the cause the check names for the same path.
func TestAStagedCopyOfTheDefaultPolicyHoldsThePathWhenNothingIsOnDisk(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	if err := os.MkdirAll(filepath.Join(repo, ".lousy-agents"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, repo, CanonicalPath, validPolicy)
	stage(t, repo, CanonicalPath)
	moveAside(t, repo, CanonicalPath)

	held, err := heldByTheIndex(NewGitRunner(), repo, "HEAD", Selection{Path: DefaultPath, Deprecated: true})

	if err != nil || held.HeldCanonical == nil || held.HeldCanonical.PolicyCause != projectreadiness.CauseStagedCopy {
		t.Fatalf("heldByTheIndex = %+v, %v; want the path held with the cause staged_copy", held, err)
	}
	if !held.CanonicalPathTaken() {
		t.Error("CanonicalPathTaken = false; a move would replace the staged copy")
	}
}

func TestAPathNothingHoldsIsLeftFree(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")

	held, err := heldByTheIndex(NewGitRunner(), repo, "HEAD", Selection{Path: DefaultPath, Deprecated: true})

	if err != nil || held.HeldCanonical != nil || held.CanonicalPathTaken() {
		t.Errorf("heldByTheIndex = %+v, %v; want the path left free", held, err)
	}
}

// A move is offered only where project.json is the policy and the disk is free, so git is asked about the index
// only there.
func TestTheIndexIsNotAskedWhereNoMoveCouldBeOffered(t *testing.T) {
	asked := false
	gitThatNoOneShouldAsk := func(string, ...string) ([]byte, error) {
		asked = true
		return nil, errBoom
	}
	selections := map[string]Selection{
		"project.json is not the policy":  {},
		"a file is on disk at the path":   {Deprecated: true, UncommittedCanonical: true},
		"an entry on disk cannot be used": {Deprecated: true, UnusableCanonical: &ConfigError{}},
	}
	for name, selection := range selections {
		t.Run(name, func(t *testing.T) { requireTheIndexIsNotAsked(t, selection, gitThatNoOneShouldAsk, &asked) })
	}
}

func requireTheIndexIsNotAsked(t *testing.T, selection Selection, run func(string, ...string) ([]byte, error), asked *bool) {
	t.Helper()
	held, err := heldByTheIndex(run, t.TempDir(), "HEAD", selection)

	if err != nil || *asked || held.HeldCanonical != nil {
		t.Errorf("heldByTheIndex(%+v) = %+v, %v, asked git: %v; want it passed on and no question", selection, held, err, *asked)
	}
}

// Git that cannot say what the index holds settles nothing: the failure is returned and the path is not read as free.
func TestAQuestionAboutTheIndexThatGitCannotAnswerIsReturnedAndNotReadAsAFreePath(t *testing.T) {
	gitThatAnswersNothing := func(string, ...string) ([]byte, error) { return nil, errBoom }

	held, err := heldByTheIndex(gitThatAnswersNothing, t.TempDir(), "HEAD", Selection{Path: DefaultPath, Deprecated: true})

	if !errors.Is(err, errBoom) {
		t.Errorf("heldByTheIndex = %+v, %v; want the failed question", held, err)
	}
}
