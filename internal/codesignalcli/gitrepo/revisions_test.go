package gitrepo

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func TestResolveBaselineRevision(t *testing.T) {
	t.Run("valid repo", resolveBaselineRevisionValidRepo)

	t.Run("non-worktree directory", resolveBaselineRevisionNonWorktree)

	t.Run("repository with no commits", resolveBaselineRevisionNoCommits)
}

func resolveBaselineRevisionValidRepo(t *testing.T) {
	dir := gitfixture.Init(t)
	headSHA := gitfixture.CommitFile(t, dir, "a.go", "package a\n")

	got, err := ResolveBaselineRevision(dir)
	if err != nil {
		t.Fatalf("ResolveBaselineRevision: unexpected error: %v", err)
	}
	if got != headSHA {
		t.Errorf("ResolveBaselineRevision() = %q, want %q", got, headSHA)
	}
}

func resolveBaselineRevisionNonWorktree(t *testing.T) {
	dir := t.TempDir()

	_, err := ResolveBaselineRevision(dir)
	if err == nil {
		t.Fatal("ResolveBaselineRevision: want error for non-worktree directory, got nil")
	}
	opErr, ok := operationalError(err)
	if !ok {
		t.Errorf("ResolveBaselineRevision error = %v, want *OperationalError", err)
	}
	if !strings.Contains(opErr.Message, "is not inside a Git worktree") {
		t.Errorf("ResolveBaselineRevision error message = %q, want worktree message", opErr.Message)
	}
}

func resolveBaselineRevisionNoCommits(t *testing.T) {
	dir := gitfixture.Init(t)

	_, err := ResolveBaselineRevision(dir)
	if err == nil {
		t.Fatal("ResolveBaselineRevision: want error for repository with no commits, got nil")
	}
	opErr, ok := operationalError(err)
	if !ok {
		t.Errorf("ResolveBaselineRevision error = %v, want *OperationalError", err)
	}
	if !strings.Contains(opErr.Message, "HEAD is not readable") {
		t.Errorf("ResolveBaselineRevision error message = %q, want HEAD-not-readable message", opErr.Message)
	}
}

func TestResolveRevisions(t *testing.T) {
	t.Run("valid base", resolveRevisionsValidBase)

	t.Run("invalid base", resolveRevisionsInvalidBase)

	t.Run("non-worktree directory", resolveRevisionsNonWorktree)

	t.Run("bare repository", resolveRevisionsBareRepository)
}

func operationalError(err error) (*OperationalError, bool) {
	opErr, ok := err.(*OperationalError)
	return opErr, ok
}

func resolveRevisionsValidBase(t *testing.T) {
	dir := gitfixture.Init(t)
	initialSHA := gitfixture.CommitFile(t, dir, "a.go", "package a\n")
	headSHA := gitfixture.CommitFile(t, dir, "b.go", "package a\n\nfunc B() {}\n")

	gotHead, gotMergeBase, err := ResolveRevisions(dir, initialSHA)
	if err != nil {
		t.Fatalf("ResolveRevisions: unexpected error: %v", err)
	}
	if gotHead != headSHA {
		t.Errorf("headSHA = %q, want %q", gotHead, headSHA)
	}
	if gotMergeBase != initialSHA {
		t.Errorf("mergeBaseSHA = %q, want %q", gotMergeBase, initialSHA)
	}
}

func resolveRevisionsInvalidBase(t *testing.T) {
	dir := gitfixture.Init(t)
	gitfixture.CommitFile(t, dir, "a.go", "package a\n")

	_, _, err := ResolveRevisions(dir, "doesnotexist12345")
	if err == nil {
		t.Fatal("ResolveRevisions: want error for unresolvable base, got nil")
	}
	if _, ok := operationalError(err); !ok {
		t.Errorf("ResolveRevisions error = %v, want *OperationalError", err)
	}
}

func resolveRevisionsNonWorktree(t *testing.T) {
	dir := t.TempDir()

	_, _, err := ResolveRevisions(dir, "HEAD")
	if err == nil {
		t.Fatal("ResolveRevisions: want error for non-worktree directory, got nil")
	}
	if _, ok := operationalError(err); !ok {
		t.Errorf("ResolveRevisions error = %v, want *OperationalError", err)
	}
}

func resolveRevisionsBareRepository(t *testing.T) {

	dir := t.TempDir()
	cmd := exec.Command("git", "init", "--bare")
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v: %s", err, output)
	}

	_, _, err := ResolveRevisions(dir, "HEAD")
	if err == nil {
		t.Fatal("ResolveRevisions: want error for bare repository, got nil")
	}
	if _, ok := operationalError(err); !ok {
		t.Errorf("ResolveRevisions error = %v, want *OperationalError", err)
	}
}
