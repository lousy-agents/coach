package codesignalcli

import (
	"os/exec"

	"testing"
)

func body_gitTest_validBase_19(t *testing.T) {
	dir := newTempGitRepoT(t)
	initialSHA := commitFileT(t, dir, "a.go", "package a\n")
	headSHA := commitFileT(t, dir, "b.go", "package a\n\nfunc B() {}\n")

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

func body_gitTest_invalidBase_36(t *testing.T) {
	dir := newTempGitRepoT(t)
	commitFileT(t, dir, "a.go", "package a\n")

	_, _, err := ResolveRevisions(dir, "doesnotexist12345")
	if err == nil {
		t.Fatal("ResolveRevisions: want error for unresolvable base, got nil")
	}
	if _, ok := operationalError(err); !ok {
		t.Errorf("ResolveRevisions error = %v, want *OperationalError", err)
	}
}

func body_gitTest_nonWorktreeDirectory_49(t *testing.T) {
	dir := t.TempDir()

	_, _, err := ResolveRevisions(dir, "HEAD")
	if err == nil {
		t.Fatal("ResolveRevisions: want error for non-worktree directory, got nil")
	}
	if _, ok := operationalError(err); !ok {
		t.Errorf("ResolveRevisions error = %v, want *OperationalError", err)
	}
}

func body_gitTest_bareRepository_61(t *testing.T) {

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
