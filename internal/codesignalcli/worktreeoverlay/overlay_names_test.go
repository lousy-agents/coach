package worktreeoverlay_test

import (
	"testing"
	"testing/fstest"

	"github.com/lousy-agents/coach/internal/codesignalcli/worktreeoverlay"
)

func TestOverlayListsEachParentDirectoryOncePerBuild(t *testing.T) {
	disk := newFakeDisk(map[string]diskEntry{
		"pkg/handlers/a.go": regular("package handlers\n"),
		"pkg/handlers/b.go": regular("package handlers\n"),
		"pkg/handlers/c.go": regular("package handlers\n"),
	})

	_, _, err := worktreeoverlay.New(committed(),
		[]string{"pkg/handlers/a.go", "pkg/handlers/b.go", "pkg/handlers/c.go"}, disk, worktreeoverlay.CaseInsensitiveNames())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	if got := disk.listings["pkg/handlers"]; got != 1 {
		t.Errorf("pkg/handlers was listed %d times for three changed files; each file is checked before and after its read, so six would mean no cache", got)
	}
}

func TestOverlayChecksExactSpellingOnlyOnACaseInsensitiveFilesystem(t *testing.T) {
	files := map[string]diskEntry{"pkg/handlers/a.go": regular("package handlers\n")}

	insensitive := newFakeDisk(files)
	if _, _, err := worktreeoverlay.New(committed(), []string{"pkg/handlers/a.go"}, insensitive, worktreeoverlay.CaseInsensitiveNames()); err != nil {
		t.Fatal(err)
	}
	sensitive := newFakeDisk(files)
	if _, _, err := worktreeoverlay.New(committed(), []string{"pkg/handlers/a.go"}, sensitive); err != nil {
		t.Fatal(err)
	}

	if len(insensitive.listings) == 0 {
		t.Error("a case-insensitive filesystem must list the directory to check the exact spelling")
	}
	if len(sensitive.listings) != 0 {
		t.Errorf("a case-sensitive filesystem needs no listing, got %v", sensitive.listings)
	}
}

func TestOverlayTreatsTheOldSpellingOfACaseOnlyRenameAsRemoved(t *testing.T) {
	disk := newFakeDisk(map[string]diskEntry{"pkg/handlers/Handlers.go": regular("package handlers // renamed\n")})
	disk.foldCase = true
	base := fstest.MapFS{"pkg/handlers/handlers.go": {Data: []byte("package handlers // committed\n")}}
	// A case-insensitive filesystem still resolves the old spelling, to the
	// renamed file. If this disk did not, the outcome below would hold without
	// the exact-spelling check this spec exists to exercise.
	if _, err := disk.Lstat("pkg/handlers/handlers.go"); err != nil {
		t.Fatalf("a case-insensitive disk must resolve the old spelling to the renamed file: %v", err)
	}

	_, outcome, err := worktreeoverlay.New(base, []string{"pkg/handlers/handlers.go", "pkg/handlers/Handlers.go"}, disk, worktreeoverlay.CaseInsensitiveNames())
	if err != nil {
		t.Fatal(err)
	}

	assertCaseOnlyRenameOutcome(t, outcome)
}

// On a case-sensitive filesystem the old spelling is simply gone, so the same
// rename gives the same outcome without the exact-spelling check.
func TestOverlayTreatsACaseOnlyRenameAsARemovalAndAnAdditionOnACaseSensitiveFilesystem(t *testing.T) {
	disk := newFakeDisk(map[string]diskEntry{"pkg/handlers/Handlers.go": regular("package handlers // renamed\n")})
	base := fstest.MapFS{"pkg/handlers/handlers.go": {Data: []byte("package handlers // committed\n")}}

	_, outcome, err := worktreeoverlay.New(base, []string{"pkg/handlers/handlers.go", "pkg/handlers/Handlers.go"}, disk)
	if err != nil {
		t.Fatal(err)
	}

	assertCaseOnlyRenameOutcome(t, outcome)
}

// git status lists an untracked directory it does not enter, such as a nested
// repository, with a trailing slash. It is not a file to read, and one such
// entry must not fail the overlay for every other path.
func TestOverlaySkipsADirectoryMarkerInsteadOfFailingForEveryOtherPath(t *testing.T) {
	disk := newFakeDisk(map[string]diskEntry{"go.mod": regular("module edited\n")})

	overlay, outcome, err := worktreeoverlay.New(committed(), []string{"pkg/go.mod/", "go.mod"}, disk)

	if err != nil {
		t.Fatalf("New rejected a directory marker: %v", err)
	}
	assertOutcome(t, outcome, worktreeoverlay.Outcome{Overlaid: []string{"go.mod"}})
	assertServedContents(t, overlay, map[string]string{"go.mod": "module edited\n"})
}

func TestOverlayStillRejectsAMalformedPathThatEndsInASlash(t *testing.T) {
	for _, bad := range []string{"/", "./", "../escape/", "a//b/", "/abs/"} {
		t.Run(bad, func(t *testing.T) {
			assertNewRejectsPath(t, bad)
		})
	}
}
