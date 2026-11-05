package main

import (
	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// committedWorkingTreeDisclosure names the dirty paths a committed scan did not
// read. A --worktree scan reads them instead, and discloses only what it did not
// read through its worktree view.
func committedWorkingTreeDisclosure(status *scanStatus, f codesignalFlags) []codesignal.Diagnostic {
	if f.worktree {
		return nil
	}
	return codesignalcli.WorkingTreeDisclosureDiagnostics(status.get())
}
