package projectconfig

import (
	"strings"
	"testing"
)

// Reviewing a staged edit with git diff --cached leads nowhere unless the message says how to keep it and how to drop
// it. Keeping is git restore with no source, which writes the staged content to disk; dropping is the restore from
// the revision, which brings the committed copy back to the index and the file. Each is a command that pastes whole
// from where it starts to the next semicolon, or to the end.
func TestAStagedEditIsKeptByRestoringItFromTheIndexAndDroppedByRestoringTheCommittedCopy(t *testing.T) {
	offer := stagedEditOffer("abc123", "cfg/p.json", false)

	const keep = "which writes the staged content to disk and not the committed copy, restore it with git restore -- ':(top,literal)cfg/p.json'; "
	const drop = "to drop it and bring back the committed copy instead, restore it with git restore --source=abc123 --staged --worktree -- ':(top,literal)cfg/p.json'"
	if !strings.Contains(offer.text, "git diff --cached") || !strings.Contains(offer.text, keep) || !strings.HasSuffix(offer.text, drop) {
		t.Errorf("offer %q; want the review with git diff --cached, the keep command ending at a semicolon, and %q at the end", offer.text, drop)
	}
}

func TestAStagedEditBehindAFileInTheWayIsKeptOrDroppedAfterTheFileIsMoved(t *testing.T) {
	offer := stagedEditOffer("abc123", "cfg/p.json", true)

	const keep = "move the file that blocks the path, then restore it with git restore -- ':(top,literal)cfg/p.json'"
	const drop = "move the file that blocks the path, then restore it with git restore --source=abc123 --staged --worktree -- ':(top,literal)cfg/p.json'"
	if !strings.Contains(offer.text, keep) || !strings.HasSuffix(offer.text, drop) {
		t.Errorf("offer %q; want it to move the file first for the keep command and for the drop command, which ends the text: the restore writes at the path", offer.text)
	}
}

func TestAStagedEditWhoseNameCannotBePrintedGetsTheStepsWithoutTheName(t *testing.T) {
	offer := stagedEditOffer("abc123", "cfg/p\x1b.json", false)

	if strings.Contains(offer.text, "\x1b") || strings.Count(offer.text, "(the name has a character that is not printed here)") != 2 {
		t.Errorf("offer %q; want both steps named and the unprintable name left out", offer.text)
	}
}
