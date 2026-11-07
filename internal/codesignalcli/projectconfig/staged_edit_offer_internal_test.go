package projectconfig

import (
	"strings"
	"testing"
)

// Reviewing a staged edit with git diff --cached leads nowhere unless the message says how to keep it: git
// restore with no source writes the staged content to disk, and git restore --source would write the committed
// copy and discard the edit.
func TestAStagedEditIsKeptByRestoringItFromTheIndex(t *testing.T) {
	offer := stagedEditOffer("cfg/p.json", false)

	const keep = "which writes the staged content to disk and not the committed copy, restore it with git restore -- ':(top,literal)cfg/p.json'"
	if !strings.Contains(offer.text, "git diff --cached") || !strings.HasSuffix(offer.text, keep) {
		t.Errorf("offer %q; want the review with git diff --cached, and the command as the tail of the text: %q", offer.text, keep)
	}
	if strings.Contains(offer.text, "--source") {
		t.Errorf("offer %q; a restore from a revision would discard the staged edit", offer.text)
	}
}

func TestAStagedEditBehindAFileInTheWayIsKeptAfterTheFileIsMoved(t *testing.T) {
	offer := stagedEditOffer("cfg/p.json", true)

	const keep = "move the file that blocks the path, then restore it with git restore -- ':(top,literal)cfg/p.json'"
	if !strings.HasSuffix(offer.text, keep) {
		t.Errorf("offer %q; want it to end with %q: the restore writes at the path, which a file stands in the way of", offer.text, keep)
	}
}

func TestAStagedEditWhoseNameCannotBePrintedGetsTheStepWithoutTheName(t *testing.T) {
	offer := stagedEditOffer("cfg/p\x1b.json", false)

	const keep = "restore it from the index (the name has a character that is not printed here)"
	if strings.Contains(offer.text, "\x1b") || !strings.HasSuffix(offer.text, keep) {
		t.Errorf("offer %q; want the step named and the unprintable name left out", offer.text)
	}
}
