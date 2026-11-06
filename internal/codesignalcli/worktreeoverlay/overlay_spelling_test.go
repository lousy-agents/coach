package worktreeoverlay_test

import (
	"testing"
	"testing/fstest"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

// Git keeps the index spelling for a case-only rename that was not staged, so
// it lists the old name as modified while the file on disk carries the new one.
// The path exists and is dirty, so it must be named, not treated as removed.
func TestOverlayNamesAPathSpelledDifferentlyOnDiskInsteadOfTreatingItAsRemoved(t *testing.T) {
	disk := newFakeDisk(map[string]diskEntry{"pkg/handlers/Handlers.go": regular("package handlers // edited\n")})
	disk.foldCase = true
	base := fstest.MapFS{"pkg/handlers/handlers.go": {Data: []byte("package handlers // committed\n")}}

	overlay, outcome, err := worktreeoverlay.New(base, []string{"pkg/handlers/handlers.go"}, disk, worktreeoverlay.CaseInsensitiveNames())
	if err != nil {
		t.Fatal(err)
	}

	assertOutcome(t, outcome, worktreeoverlay.Outcome{
		Unreadable: []worktreeoverlay.Unreadable{{Path: "pkg/handlers/handlers.go", Reason: worktreeoverlay.ReasonNameMismatch}},
	})
	assertAllAbsent(t, overlay, []string{"pkg/handlers/handlers.go"})
}

func TestOverlayNamesAPathWhoseDirectoryIsSpelledDifferentlyOnDisk(t *testing.T) {
	disk := newFakeDisk(map[string]diskEntry{"pkg/Handlers/handlers.go": regular("package handlers // edited\n")})
	disk.foldCase = true
	base := fstest.MapFS{"pkg/handlers/handlers.go": {Data: []byte("package handlers // committed\n")}}

	_, outcome, err := worktreeoverlay.New(base, []string{"pkg/handlers/handlers.go"}, disk, worktreeoverlay.CaseInsensitiveNames())
	if err != nil {
		t.Fatal(err)
	}

	assertOutcome(t, outcome, worktreeoverlay.Outcome{
		Unreadable: []worktreeoverlay.Unreadable{{Path: "pkg/handlers/handlers.go", Reason: worktreeoverlay.ReasonNameMismatch}},
	})
}

// Git records a composed name, so a decomposed one on disk has no other path
// Git lists in its place, and an untracked file is named too, not dropped.
func TestOverlayNamesAnUntrackedPathWhoseUnicodeSpellingOnDiskIsDecomposed(t *testing.T) {
	disk := newFakeDisk(map[string]diskEntry{"pkg/handlers/café.go": regular("package handlers\n")})
	disk.foldCase = true

	_, outcome, err := worktreeoverlay.New(committed(), []string{"pkg/handlers/café.go"}, disk, worktreeoverlay.CaseInsensitiveNames())
	if err != nil {
		t.Fatal(err)
	}

	assertOutcome(t, outcome, worktreeoverlay.Outcome{
		Unreadable: []worktreeoverlay.Unreadable{{Path: "pkg/handlers/café.go", Reason: worktreeoverlay.ReasonNameMismatch}},
	})
}
