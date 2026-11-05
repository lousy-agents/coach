package main

import "github.com/lousy-agents/coach/internal/codesignalcli"

// scanStatus is the working-tree status of one scan, read the first time a part of the scan asks for it and never again,
// so that every part describes the same tree. A committed scan asks no sooner than it always has: a policy it refuses
// ends it before any `git status` runs, and `git status` can run a clean filter that a repository configures on a file
// it has to hash. A --worktree scan asks first, for the view it reads the tree through.
type scanStatus struct {
	dir      string
	worktree bool
	read     bool
	status   codesignalcli.WorktreeStatus
}

func newScanStatus(dir string, f codesignalFlags) *scanStatus {
	return &scanStatus{dir: dir, worktree: f.worktree}
}

func (s *scanStatus) get() codesignalcli.WorktreeStatus {
	if !s.read {
		s.status, s.read = codesignalcli.ReadWorktreeStatus(s.dir, s.worktree), true
	}
	return s.status
}
