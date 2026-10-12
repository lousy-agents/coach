package gitrepo

import (
	"os/exec"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

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
