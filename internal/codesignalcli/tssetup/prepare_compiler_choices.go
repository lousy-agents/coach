package tssetup

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectcheck"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

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
// projectcheck.Run' own verification data: action.Choices when the
// package-manager-adapter restriction already populated it, or each mise
// scope's own projectreadiness.MiseChoice.Verified (miseSetupChoicesForReadiness,
// the same tstoolchain.EvaluateMiseSetupChoices call projectcheck.Run itself
// makes) when it did not -- action.Choices is nil exactly when no adapter
// rejection has restricted it yet, not when nothing is offered (see
// restrictPrepareCompilerChoices' own contract).
func miseChoicesForPrepareCompiler(dir, revision string, policy policyref.Ref, action projectreadiness.NextAction) []string {
	if action.Choices != nil {
		return filterMiseChoiceKinds(action.Choices)
	}
	var offered []string
	for _, choice := range miseSetupChoicesForReadiness(dir, revision, policy) {
		if choice.Verified {
			offered = append(offered, choice.Kind)
		}
	}
	return offered
}

// filterMiseChoiceKinds keeps only the mise origins this flow handles.
// action.Choices may also name a project-package-manager choice from a
// sibling task; that choice is a different installation-choice kind
// entirely, not this flow's concern.
func filterMiseChoiceKinds(choices []string) []string {
	var mise []string
	for _, choice := range choices {
		if choice == tstoolchain.OriginMiseProject || choice == tstoolchain.OriginMiseGlobal {
			mise = append(mise, choice)
		}
	}
	return mise
}

// miseSetupChoicesForReadiness reuses the policy path and roots
// projectcheck.Run just checked. A discovered policy is the one Discover selects.
func miseSetupChoicesForReadiness(dir, revision string, policy policyref.Ref) []projectreadiness.MiseChoice {
	roots, err := checkedPolicyRoots(dir, revision, policy)
	if err != nil {
		return nil
	}
	return tstoolchain.EvaluateMiseSetupChoices(dir, roots)
}

func checkedPolicyRoots(dir, revision string, policy policyref.Ref) ([]string, error) {
	policyPath, err := projectcheck.ResolvePolicyPath(dir, revision, policy)
	if err != nil {
		return nil, err
	}
	_, roots, err := projectcheck.PolicyChecker(policy)(dir, revision, policyPath)
	return roots, err
}
