package worktreeoverlay_test

import (
	"io/fs"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

func TestOverlayDoesNotRemoveADirectoryTheDiskLacksAsIfItWereATrackedFile(t *testing.T) {
	overlay, outcome, err := worktreeoverlay.New(committed(), []string{"pkg/db"}, newFakeDisk(nil))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if len(outcome.Removed) != 0 {
		t.Errorf("Removed = %v, want none: pkg/db is a directory in the committed snapshot, not a tracked file gone from disk", outcome.Removed)
	}
	entries, err := fs.ReadDir(overlay, "pkg/db")
	if err != nil || len(entries) != 1 || entries[0].Name() != "db.go" {
		t.Errorf("ReadDir(pkg/db) = %v, %v; want the committed db.go still listed", entries, err)
	}
}
