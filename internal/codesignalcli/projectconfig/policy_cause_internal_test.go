package projectconfig

import (
	"fmt"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// Every offer that names no remedy has a word for why. A verdict added with an offer
// and without a word would reach a caller of --check-project as a bare policy_missing.
func TestEveryOfferThatNamesNoRemedyNamesItsCause(t *testing.T) {
	for verdict := restoreVerdict(0); verdict < restoreVerdictCount; verdict++ {
		runBlockedAndOpen(t, verdict)
	}
}

func runBlockedAndOpen(t *testing.T, verdict restoreVerdict) {
	t.Helper()
	for _, blocked := range []bool{false, true} {
		t.Run(fmt.Sprintf("verdict %d, a file in the way = %v", verdict, blocked), func(t *testing.T) { requireTheCauseMatchesTheOffer(t, verdict, blocked) })
	}
}

func requireTheCauseMatchesTheOffer(t *testing.T, verdict restoreVerdict, blocked bool) {
	t.Helper()
	judgment := verdictCase{verdict: verdict, failureForTheOffer: true}.judgment()
	cause := [2]MissingCause{CauseAbsent, CauseBlockedParent}[blockedIndex(blocked)]

	offer, offered := restoreOfferFor(judgment, "HEAD", &ConfigError{Kind: KindMissing, Cause: cause}, "cfg/p.json")

	word := offerCause(judgment, blocked)
	if offered && offer.remedy == projectreadiness.RemedyNone && (word == "" || word == projectreadiness.CauseUndetermined) {
		t.Errorf("the offer %q names no remedy and no cause: offerCause = %q", offer.text, word)
	}
}

func TestOfferCauseOrdersTheLockAndTheFileInTheWayBeforeTheVerdict(t *testing.T) {
	cases := map[string]struct {
		judgment policyJudgment
		blocked  bool
		want     projectreadiness.PolicyCause
	}{
		"a locked index withholds a restore that would overwrite nothing": {policyJudgment{verdict: restoreOverwritesNothing, indexLocked: true}, false, projectreadiness.CauseIndexLocked},
		"a file in the way outranks a lock it cannot get to":              {policyJudgment{verdict: restoreOverwritesNothing, indexLocked: true}, true, projectreadiness.CauseParentNotDirectory},
		"a file in the way of a restore over an empty entry":              {policyJudgment{verdict: restoreOverwritesEmptyEntry}, true, projectreadiness.CauseParentNotDirectory},
		"a staged edit keeps its own word when a file is in the way":      {policyJudgment{verdict: restoreDiscardsStagedEdit}, true, projectreadiness.CauseStagedEdit},
		"a verdict the offers do not know":                                {policyJudgment{verdict: unknownVerdict}, false, projectreadiness.CauseUndetermined},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireOfferCause(t, c.judgment, c.blocked, c.want) })
	}
}

func requireOfferCause(t *testing.T, judgment policyJudgment, blocked bool, want projectreadiness.PolicyCause) {
	t.Helper()
	if got := offerCause(judgment, blocked); got != want {
		t.Errorf("offerCause(%+v, blocked %v) = %q, want %q", judgment, blocked, got, want)
	}
}
