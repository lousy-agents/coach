package codesignalcli

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/prompt"
)

// promptForMiseSetupChoice requires the user to type one offered choice's
// exact name, or "cancel". There is no numbered/default selection: an
// unrecognized or blank answer cancels rather than falling back to any
// choice, including when only one is offered.
func promptForMiseSetupChoice(out io.Writer, reader *bufio.Reader, choices []string) (choice string, cancelled bool) {
	fmt.Fprintln(out, "TypeScript compiler setup: the following mise installation choices are executable and verified in this environment:")
	for _, c := range choices {
		fmt.Fprintf(out, "  - %s\n", c)
	}
	fmt.Fprintln(out, "Type the exact choice name to select it, or 'cancel' to cancel without making any change. There is no default: an unrecognized or blank answer cancels.")
	fmt.Fprint(out, "> ")
	answer, unreadable := prompt.ReadLine(reader)
	if unreadable {
		return "", true
	}
	answer = strings.TrimSpace(answer)
	for _, c := range choices {
		if answer == c {
			return c, false
		}
	}
	return "", true
}

func prepareCompilerNextAction(readiness *projectreadiness.Result) (projectreadiness.NextAction, bool) {
	if readiness == nil {
		return projectreadiness.NextAction{}, false
	}
	for _, action := range readiness.NextActions {
		if action.Kind == projectreadiness.NextActionPrepareCompiler {
			return action, true
		}
	}
	return projectreadiness.NextAction{}, false
}

// miseChoicesForPrepareCompiler names which of mise_project/mise_global are
// genuinely offered for action, consuming rather than re-deriving
// CheckProjectReadiness' own verification data: action.Choices when the
// package-manager-adapter restriction already populated it, or each mise
// scope's own projectreadiness.MiseChoice.Verified (miseSetupChoicesForReadiness,
// the same evaluateMiseSetupChoices call CheckProjectReadiness itself
// makes) when it did not -- action.Choices is nil exactly when no adapter
// rejection has restricted it yet, not when nothing is offered (see
// restrictPrepareCompilerChoices' own contract).
func miseChoicesForPrepareCompiler(dir, revision, configPath string, action projectreadiness.NextAction) []string {
	if action.Choices != nil {
		return filterMiseChoiceKinds(action.Choices)
	}
	var offered []string
	for _, choice := range miseSetupChoicesForReadiness(dir, revision, configPath) {
		if choice.Verified {
			offered = append(offered, choice.Kind)
		}
	}
	return offered
}
