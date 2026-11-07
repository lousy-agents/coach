package projectreadiness

import "testing"

// Restoring over an empty index entry is a restore: it names the same next action, and as a remedy it
// is the answer, so a gap that carries it carries no cause. It is its own remedy only so the sentence
// that explains the restore can say why it loses nothing.
func TestARestoreOverAnEmptyIndexEntryIsARestore(t *testing.T) {
	action, named := NextActionForRemedy(RemedyRestoreOverEmptyEntry)
	if action != NextActionRestorePolicy || !named {
		t.Errorf("NextActionForRemedy = %q, %v; want %q", action, named, NextActionRestorePolicy)
	}
	gapAction, gapNamed := NextActionForGap(GapPolicyMissing, RemedyRestoreOverEmptyEntry)
	if gapAction != NextActionRestorePolicy || !gapNamed {
		t.Errorf("NextActionForGap = %q, %v; want %q", gapAction, gapNamed, NextActionRestorePolicy)
	}
	check := Check{Remedy: RemedyRestoreOverEmptyEntry}.WithCause("why", CauseDirectory)
	if check.CauseText != "" || check.PolicyCause != "" {
		t.Errorf("check = %+v; a remedy names what to do, so it carries no cause", check)
	}
}
