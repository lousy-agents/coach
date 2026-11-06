package projectconfig

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A locked index makes every printed git restore fail, so the lock is checked for
// the verdicts whose offer prints one and for no others. offersRestoreCommand is
// that list; this walks every verdict to hold it to what the offers print, so a
// verdict that gains a restore command without joining the list, or stays in the
// list after losing its command, fails here instead of printing a command that
// fails or checking a lock that cannot matter.
func TestALockedIndexWithholdsExactlyTheOffersThatPrintARestore(t *testing.T) {
	for verdict := restoreVerdict(0); verdict < restoreVerdictCount; verdict++ {
		t.Run(fmt.Sprintf("verdict %d", verdict), func(t *testing.T) { assertTheLockAndTheOfferAgree(t, verdict) })
	}
}

func offerForVerdict(judgment policyJudgment) restoreOffer {
	offer, _ := restoreOfferFor(judgment, "HEAD", &ConfigError{Kind: KindMissing, Cause: CauseAbsent}, "cfg/p.json")
	return offer
}

func assertTheLockAndTheOfferAgree(t *testing.T, verdict restoreVerdict) {
	t.Helper()
	judgment := verdictCase{verdict: verdict, failureForTheOffer: true}.judgment()

	printsRestore := strings.Contains(offerForVerdict(judgment).text, "git restore")

	if printsRestore != offersRestoreCommand(verdict) {
		t.Fatalf("the offer prints a git restore command = %v, but offersRestoreCommand = %v: the lock check and the offers disagree",
			printsRestore, offersRestoreCommand(verdict))
	}
	if printsRestore {
		assertALockedIndexWithholdsTheRestore(t, judgment)
	}
}

func assertALockedIndexWithholdsTheRestore(t *testing.T, judgment policyJudgment) {
	t.Helper()
	judgment.indexLocked = true

	locked := offerForVerdict(judgment)

	saysWhy := strings.Contains(locked.text, "locked") && !strings.Contains(locked.text, "git restore")
	if !saysWhy || locked.remedy != projectreadiness.RemedyNone {
		t.Errorf("a locked index must withhold the restore, offer no remedy and say why; got remedy %d, %q", locked.remedy, locked.text)
	}
}
