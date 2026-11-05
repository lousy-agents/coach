package main

import "github.com/lousy-agents/coach/internal/codesignalcli"

// worktreeViewFor is the view a --worktree scan reads the working tree through,
// built on the status the scan read once; a committed scan has none.
func worktreeViewFor(status codesignalcli.WorktreeStatus, f codesignalFlags) *codesignalcli.WorktreeView {
	if !f.worktree {
		return nil
	}
	view := codesignalcli.NewWorktreeView(status)
	return &view
}
