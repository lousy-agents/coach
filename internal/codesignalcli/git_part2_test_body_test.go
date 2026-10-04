package codesignalcli

import (
	"strings"
	"testing"
)

func body_gitPart2Test_validRepo_9(t *testing.T) {
	dir := newTempGitRepoT(t)
	headSHA := commitFileT(t, dir, "a.go", "package a\n")

	got, err := ResolveBaselineRevision(dir)
	if err != nil {
		t.Fatalf("ResolveBaselineRevision: unexpected error: %v", err)
	}
	if got != headSHA {
		t.Errorf("ResolveBaselineRevision() = %q, want %q", got, headSHA)
	}
}

func body_gitPart2Test_nonWorktreeDirectory_22(t *testing.T) {
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

func body_gitPart2Test_repositoryWithNoCommits_38(t *testing.T) {
	dir := newTempGitRepoT(t)

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

func body_gitPart2Test_68(t *testing.T, tt struct {
	name string
	path string
}) {
	payload := joinNUL("M", tt.path)
	got, err := parseNameStatusZ(payload)
	if err != nil {
		t.Fatalf("parseNameStatusZ: unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].paths[0] != tt.path {
		t.Fatalf("parseNameStatusZ(%q) = %#v, want single record with path %q", payload, got, tt.path)
	}
}
