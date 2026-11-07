package projectconfig

import (
	"strings"
	"testing"
)

// A policy in a merge conflict is missing from disk (that is the state the message describes), so there is no file
// to edit, and git add on a path that is not there records a deletion and drops the other side. The message
// brings the file back with its conflict markers first.
func TestTheMergeConflictOffersBringTheConflictedFileBackBeforeAskingForItToBeResolved(t *testing.T) {
	offers := map[string]restoreOffer{
		"over a committed policy":      mergeConflictOffer("cfg/p.json"),
		"for a policy nothing commits": unmergedNotCommittedOffer("cfg/p.json"),
	}
	for name, offer := range offers {
		t.Run(name, func(t *testing.T) { requireTheFileIsBroughtBack(t, offer) })
	}
}

func requireTheFileIsBroughtBack(t *testing.T, offer restoreOffer) {
	t.Helper()
	const command = "git checkout --merge -- ':(top,literal)cfg/p.json'"
	if !strings.Contains(offer.text, command) || !strings.Contains(offer.text, "resolve the conflict markers in it, and git add it") {
		t.Errorf("offer %q; want the command %q and then the resolution", offer.text, command)
	}
	if strings.Contains(offer.text, "edit the file") || strings.Contains(offer.text, "git status") {
		t.Errorf("offer %q; the file is gone, so it cannot be edited, and git status resolves nothing", offer.text)
	}
}

func TestAConflictedNameThatCannotBePrintedGetsTheStepWithoutTheCommand(t *testing.T) {
	offer := mergeConflictOffer("cfg/p\x1b.json")

	if strings.Contains(offer.text, "\x1b") || !strings.Contains(offer.text, "git checkout --merge (the name has a character that is not printed here)") {
		t.Errorf("offer %q; want the step named and the unprintable name left out", offer.text)
	}
}
