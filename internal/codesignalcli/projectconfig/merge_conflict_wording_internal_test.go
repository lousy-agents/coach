package projectconfig

import (
	"strings"
	"testing"
)

// git status lists the files with an unresolved merge conflict and resolves nothing, so it is not the step the
// message sends the user to; the step is to resolve the conflict in the file and add it.
func TestTheMergeConflictOffersNameTheStepThatResolvesIt(t *testing.T) {
	offers := map[string]restoreOffer{
		"over a committed policy":      mergeConflictOffer,
		"for a policy nothing commits": unmergedNotCommittedOffer,
	}
	for name, offer := range offers {
		t.Run(name, func(t *testing.T) { requireTheStepThatResolvesIt(t, offer) })
	}
}

func requireTheStepThatResolvesIt(t *testing.T, offer restoreOffer) {
	t.Helper()
	if !strings.Contains(offer.text, "git add") || strings.Contains(offer.text, "resolve the conflict with git status") {
		t.Errorf("offer %q; want it to send the user to resolve the conflict in the file and git add it", offer.text)
	}
}
