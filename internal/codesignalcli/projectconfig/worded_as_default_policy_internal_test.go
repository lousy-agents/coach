package projectconfig

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

const namedMessage = `coach codesignal: --project-config ".lousy-agents/coach.json" exists in the worktree but is not committed at revision "abc" (project_config_invalid): commit the file so it is readable at the analyzed revision`

func TestAPolicyCoachFoundIsWordedAsTheDefaultPolicyAndNeverAsAFlag(t *testing.T) {
	got := WordedAsTheDefaultPolicy(namedMessage, projectreadiness.CauseChangedOnDisk, ".lousy-agents/coach.json")

	if strings.Contains(got, "--project-config") || !strings.Contains(got, `the default policy ".lousy-agents/coach.json" exists in the worktree`) {
		t.Errorf("message %q; want the default policy named and no flag", got)
	}
	if !strings.Contains(got, "(project_config_invalid)") {
		t.Errorf("message %q; want the refusal's code kept for a cause that is a refusal", got)
	}
}

// A file that is only not committed is no refusal where Coach found the policy: the scan goes on and says so with a
// project_config_uncommitted diagnostic, so the reason carries that word.
func TestAnUncommittedPolicyCoachFoundCarriesTheScansWordAndNotTheCodeOfARefusal(t *testing.T) {
	got := WordedAsTheDefaultPolicy(namedMessage, projectreadiness.CauseNotCommitted, ".lousy-agents/coach.json")

	if !strings.Contains(got, "(project_config_uncommitted)") || strings.Contains(got, "project_config_invalid") {
		t.Errorf("message %q; want project_config_uncommitted and no refusal code", got)
	}
}

func TestOnlyTheFirstMentionOfTheFlagAndAnotherPathAreLeftAlone(t *testing.T) {
	message := `coach codesignal: --project-config "other.json" is invalid (project_config_invalid)`

	if got := WordedAsTheDefaultPolicy(message, projectreadiness.CauseNotCommitted, ".lousy-agents/coach.json"); strings.Contains(got, "the default policy") {
		t.Errorf("message %q; the flag named another path, so it is not reworded", got)
	}
}

func TestTheScansWordingAndTheReadinessWordingOfOneErrorAreTheSame(t *testing.T) {
	err := &ConfigError{Kind: KindUncommitted, PolicyCause: projectreadiness.CauseNotCommitted, Message: namedMessage}

	scan := asDefaultPolicyError(err, ".lousy-agents/coach.json")

	if reworded := configErrorOf(t, scan).Message; reworded != WordedAsTheDefaultPolicy(namedMessage, projectreadiness.CauseNotCommitted, ".lousy-agents/coach.json") {
		t.Errorf("scan message %q differs from the readiness wording", reworded)
	}
}
