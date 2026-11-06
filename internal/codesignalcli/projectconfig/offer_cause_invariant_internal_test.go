package projectconfig

import (
	"fmt"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// An offer names a remedy or says why it names none: never both, and never neither. The cause is
// set next to the remedy in each offer, so the two cannot be decided apart, and this holds every
// offer to it, with and without a file in the way and with and without a lock on the index.
func TestAnOfferNamesARemedyOrACauseNeverBothAndNeverNeither(t *testing.T) {
	for verdict := restoreVerdict(0); verdict < restoreVerdictCount; verdict++ {
		runEveryVariantOfTheOffer(t, verdict)
	}
}

func runEveryVariantOfTheOffer(t *testing.T, verdict restoreVerdict) {
	t.Helper()
	variants := []struct{ blocked, locked bool }{{false, false}, {true, false}, {false, true}, {true, true}}
	for _, v := range variants {
		t.Run(fmt.Sprintf("verdict %d, a file in the way = %v, a locked index = %v", verdict, v.blocked, v.locked), func(t *testing.T) {
			requireARemedyXorACause(t, verdict, v.blocked, v.locked)
		})
	}
}

func requireARemedyXorACause(t *testing.T, verdict restoreVerdict, blocked, locked bool) {
	t.Helper()
	judgment := verdictCase{verdict: verdict, failureForTheOffer: true}.judgment()
	judgment.indexLocked = locked
	cause := [2]MissingCause{MissingAbsent, MissingBlockedParent}[blockedIndex(blocked)]

	offer, offered := restoreOfferFor(judgment, "HEAD", &ConfigError{Kind: KindMissing, Missing: cause}, "cfg/p.json")

	namesARemedy := offer.remedy != projectreadiness.RemedyNone
	if offered && namesARemedy == (offer.cause != "") {
		t.Errorf("offer %q names remedy %d and cause %q: it must name exactly one of them", offer.text, offer.remedy, offer.cause)
	}
}
