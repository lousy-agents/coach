package gitrepo

import (
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
