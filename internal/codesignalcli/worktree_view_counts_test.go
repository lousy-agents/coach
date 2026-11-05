package codesignalcli

import (
	"errors"
	"testing"
	"testing/fstest"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func statusFailures(diagnostics []codesignal.Diagnostic) []codesignal.Diagnostic {
	var failures []codesignal.Diagnostic
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == codesignal.DiagKindWorktreeStatusCheckFailed {
			failures = append(failures, diagnostic)
		}
	}
	return failures
}

// One failed status read is one failure: the counts, the changed paths and the
// disclosure all derive from it, so none of them reports it again or reports
// zeros.
func TestWorktreeViewReportsAFailedStatusReadOnceInsteadOfZeros(t *testing.T) {
	view := newWorktreeViewForTest(t, worktreeViewDeps{
		status:    WorktreeStatus{Err: errors.New("status timed out")},
		committed: fstest.MapFS{"a.go": {Data: []byte("package a\n")}},
		disk:      absentDisk{},
	})

	diagnostics := view.Disclosure(".", "HEAD", "")

	if failures := statusFailures(diagnostics); len(failures) != 1 || len(diagnostics) != 1 {
		t.Fatalf("a failed status read gave %d status-failure diagnostics among %d, want exactly 1: %+v", len(failures), len(diagnostics), diagnostics)
	}
}

func TestWorktreeViewReportsNoFailureWhenTheStatusReadSucceeds(t *testing.T) {
	view := newWorktreeViewForTest(t, worktreeViewDeps{
		status:    statusFromPorcelain(nil, nil),
		committed: fstest.MapFS{"a.go": {Data: []byte("package a\n")}},
		disk:      absentDisk{},
	})
	if _, err := view.OpenHead(".", "HEAD"); err != nil {
		t.Fatalf("OpenHead: %v", err)
	}

	if failures := statusFailures(view.Disclosure(".", "HEAD", "")); len(failures) != 0 {
		t.Errorf("unexpected status-failure diagnostics: %+v", failures)
	}
}
