package projectconfig

import (
	"strings"
	"testing"
)

const namedMessage = `coach codesignal: --project-config ".lousy-agents/coach.json" exists in the worktree but is not committed at revision "abc" (project_config_invalid): commit the file so it is readable at the analyzed revision`

func TestAPolicyCoachFoundIsWordedAsTheDefaultPolicyAndNeverAsAFlag(t *testing.T) {
	got := WordedAsTheDefaultPolicy(namedMessage, ".lousy-agents/coach.json")

	if strings.Contains(got, "--project-config") || !strings.Contains(got, `the default policy ".lousy-agents/coach.json" exists in the worktree`) {
		t.Errorf("message %q; want the default policy named and no flag", got)
	}
	if !strings.Contains(got, "(project_config_invalid)") {
		t.Errorf("message %q; want the refusal's code kept", got)
	}
}

func TestOnlyTheFirstMentionOfTheFlagAndAnotherPathAreLeftAlone(t *testing.T) {
	message := `coach codesignal: --project-config "other.json" is invalid (project_config_invalid)`

	if got := WordedAsTheDefaultPolicy(message, ".lousy-agents/coach.json"); strings.Contains(got, "the default policy") {
		t.Errorf("message %q; the flag named another path, so it is not reworded", got)
	}
}

func TestTheScansWordingAndTheReadinessWordingOfOneErrorAreTheSame(t *testing.T) {
	err := &ConfigError{Kind: KindUncommitted, Message: namedMessage}

	scan := asDefaultPolicyError(err, ".lousy-agents/coach.json")

	if reworded := configErrorOf(t, scan).Message; reworded != WordedAsTheDefaultPolicy(namedMessage, ".lousy-agents/coach.json") {
		t.Errorf("scan message %q differs from the readiness wording", reworded)
	}
}
