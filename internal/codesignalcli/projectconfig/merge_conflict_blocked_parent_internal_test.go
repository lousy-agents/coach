package projectconfig

import (
	"strings"
	"testing"
)

const theMove = "move the file that blocks the path, then "

// A pathspec checkout is forced, and when an untracked file stands where a parent directory of the policy belongs,
// git unlinks that file to make the directory. So where a file is in the way, each checkout the advice offers is
// behind the step that moves it, for a policy that is committed and for one that nothing commits, whichever sides
// of the conflict still have the file. Accepting a deletion with git rm touches the index entry alone and is not.
func TestEveryCheckoutOfAConflictedPolicyIsBehindTheMoveWhereAFileIsInTheWay(t *testing.T) {
	for _, sides := range []conflictSides{bothSides, onlyOurs, onlyTheirs} {
		assertEveryCheckoutBehindTheMove(t, "over a committed policy", mergeConflictOffer("cfg/p.json", sides, true), sides)
		assertEveryCheckoutBehindTheMove(t, "for a policy nothing commits", unmergedNotCommittedOffer("cfg/p.json", sides, true), sides)
	}
}

func assertEveryCheckoutBehindTheMove(t *testing.T, name string, offer restoreOffer, sides conflictSides) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		text := offer.text
		at := strings.Index(text, "git checkout")
		if at < 0 || !strings.Contains(text[:at], theMove) || strings.Count(text, "git checkout") != strings.Count(text, theMove) {
			t.Errorf("sides %+v: offer %q; want each git checkout behind %q", sides, text, theMove)
		}
	})
}

func TestTheDeletionIsNotPutBehindTheMoveAndTheCheckoutIsWhereNoFileIsInTheWay(t *testing.T) {
	blocked := mergeConflictOffer("cfg/p.json", onlyOurs, true).text
	free := mergeConflictOffer("cfg/p.json", onlyOurs, false).text

	if i := strings.Index(blocked, "or accept the deletion with git rm"); i < 0 || strings.Contains(blocked[i:], theMove) {
		t.Errorf("offer %q; want the deletion offered without the move, which it does not need", blocked)
	}
	if strings.Contains(free, theMove) {
		t.Errorf("offer %q; want no move where no file is in the way", free)
	}
	if bothDeleted := unmergedNotCommittedOffer("cfg/p.json", neitherSide, true).text; strings.Contains(bothDeleted, theMove) {
		t.Errorf("offer %q; want no move for a deletion that is accepted with git rm alone", bothDeleted)
	}
}

// restoreOfferFor passes the state of the parent to both offers, so it is the dispatch that is held here.
func TestARestoreOfferForAConflictedPolicyKnowsAboutTheFileInTheWay(t *testing.T) {
	blockedParent := &ConfigError{Kind: KindMissing, Missing: MissingBlockedParent}
	for verdict, name := range map[restoreVerdict]string{restoreMergeConflict: "committed", restoreUnmergedNotCommitted: "not committed"} {
		offer, ok := restoreOfferFor(policyJudgment{verdict: verdict, conflictSides: bothSides}, "rev", blockedParent, "cfg/p.json")

		if !ok || !strings.Contains(offer.text, theMove+"bring the conflicted file back with git checkout --merge") {
			t.Errorf("%s: offer %q; want the checkout behind the move", name, offer.text)
		}
	}
}
