package worktreeoverlay_test

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

// renamedDirectoryDisk holds pkg/db renamed to pkg/DB, on a disk that resolves
// the old spelling to the renamed directory as a case-insensitive filesystem
// does.
func renamedDirectoryDisk(t *testing.T) *fakeDisk {
	t.Helper()
	disk := newFakeDisk(map[string]diskEntry{"pkg/DB/db.go": regular("package db // renamed\n")})
	disk.foldCase = true
	// If this disk did not resolve the old spelling, the outcome below would
	// hold without the exact-spelling check the test exists to exercise.
	if _, err := disk.Lstat("pkg/db/db.go"); err != nil {
		t.Fatalf("a case-insensitive disk must resolve the old directory spelling to the renamed directory: %v", err)
	}
	return disk
}

func TestOverlayTreatsTheOldSpellingOfACaseOnlyDirectoryRenameAsRemoved(t *testing.T) {
	_, outcome, err := worktreeoverlay.New(committed(), []string{"pkg/db/db.go", "pkg/DB/db.go"}, renamedDirectoryDisk(t), worktreeoverlay.CaseInsensitiveNames())
	if err != nil {
		t.Fatal(err)
	}

	assertOutcome(t, outcome, worktreeoverlay.Outcome{
		Overlaid: []string{"pkg/DB/db.go"},
		Removed:  []string{"pkg/db/db.go"},
	})
}

func TestOverlayTreatsTheOldSpellingOfACaseOnlyRenameOfAHigherDirectoryAsRemoved(t *testing.T) {
	disk := newFakeDisk(map[string]diskEntry{"Pkg/handlers/h.go": regular("package handlers // renamed\n")})
	disk.foldCase = true

	_, outcome, err := worktreeoverlay.New(committed(), []string{"pkg/handlers/h.go", "Pkg/handlers/h.go"}, disk, worktreeoverlay.CaseInsensitiveNames())
	if err != nil {
		t.Fatal(err)
	}

	assertOutcome(t, outcome, worktreeoverlay.Outcome{
		Overlaid: []string{"Pkg/handlers/h.go"},
		Removed:  []string{"pkg/handlers/h.go"},
	})
}
