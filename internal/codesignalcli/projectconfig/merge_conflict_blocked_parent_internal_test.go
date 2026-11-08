package projectconfig

import (
	"strings"
	"testing"
)

const theMove = "move the file that blocks the path, then "

// Bringing a file back writes the policy's path, and when an untracked file stands where a parent directory of the policy
// belongs, git unlinks that file to make the directory. So where a file is in the way, each step that writes the path
// is behind the step that moves it, for a policy that is committed and for one that nothing commits, whichever sides of
// the conflict still have the file. Accepting a deletion touches the index entry alone and is not.
func TestEveryBringingBackOfAConflictedPolicyIsBehindTheMoveWhereAFileIsInTheWay(t *testing.T) {
	for _, sides := range []conflictSides{bothSides, onlyOurs, onlyTheirs} {
		assertEveryBringingBackBehindTheMove(t, "over a committed policy", mergeConflictOffer("cfg/p.json", sides, true), sides)
		assertEveryBringingBackBehindTheMove(t, "for a policy nothing commits", unmergedNotCommittedOffer("cfg/p.json", sides, true), sides)
	}
}

func assertEveryBringingBackBehindTheMove(t *testing.T, name string, offer restoreOffer, sides conflictSides) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		text := offer.text
		steps := strings.Count(text, "bring back ") + strings.Count(text, "by bringing it back")
		if steps != 1 || strings.Count(text, theMove) != 1 || !strings.Contains(text, theMove+"bring back ") && !strings.Contains(text, theMove+"keep the version of ") {
			t.Errorf("sides %+v: offer %q; want the one step that writes the path behind %q", sides, text, theMove)
		}
	})
}

func TestTheDeletionIsNotPutBehindTheMoveAndTheStepIsWhereNoFileIsInTheWay(t *testing.T) {
	blocked := mergeConflictOffer("cfg/p.json", onlyOurs, true).text
	free := mergeConflictOffer("cfg/p.json", onlyOurs, false).text

	if i := strings.Index(blocked, "or accept the deletion by recording"); i < 0 || strings.Contains(blocked[i:], theMove) {
		t.Errorf("offer %q; want the deletion offered without the move, which it does not need", blocked)
	}
	if strings.Contains(free, theMove) {
		t.Errorf("offer %q; want no move where no file is in the way", free)
	}
	if bothDeleted := unmergedNotCommittedOffer("cfg/p.json", neitherSide, true).text; strings.Contains(bothDeleted, theMove) {
		t.Errorf("offer %q; want no move for a deletion that only records the index entry", bothDeleted)
	}
}

// restoreOfferFor passes the state of the parent to both offers, so it is the dispatch that is held here.
func TestARestoreOfferForAConflictedPolicyKnowsAboutTheFileInTheWay(t *testing.T) {
	blockedParent := &ConfigError{Kind: KindMissing, Missing: MissingBlockedParent}
	for verdict, name := range map[restoreVerdict]string{restoreMergeConflict: "committed", restoreUnmergedNotCommitted: "not committed"} {
		offer, ok := restoreOfferFor(policyJudgment{verdict: verdict, conflictSides: bothSides}, "rev", blockedParent, "cfg/p.json")

		if !ok || !strings.Contains(offer.text, theMove+"bring back "+conflictedFile+" with the conflict markers") {
			t.Errorf("%s: offer %q; want the step behind the move", name, offer.text)
		}
	}
}
