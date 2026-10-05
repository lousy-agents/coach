package projectcheck

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// restrictPrepareCompilerChoices: once the project package-manager adapter
// has actually been evaluated and rejected (checks.PackageManager.State ==
// projectreadiness.Fail, passed as adapterRejected), a rejected installation choice
// never appears in prepare_compiler's Choices, and prepare_compiler is
// withheld entirely when no verified installation choice remains. An
// adapter that has not been evaluated (projectreadiness.NotChecked -- no recognized
// packageManager field or lockfile at all) must never by itself count as
// "no installation choice exists" -- only a genuine rejection does, so this
// is a no-op unless adapterRejected is true.
func restrictPrepareCompilerChoices(actions []projectreadiness.NextAction, adapterRejected bool, verified []string) []projectreadiness.NextAction {
	if !adapterRejected {
		return actions
	}
	restricted := make([]projectreadiness.NextAction, 0, len(actions))
	for _, action := range actions {
		if action.Kind != projectreadiness.NextActionPrepareCompiler {
			restricted = append(restricted, action)
			continue
		}
		if len(verified) == 0 {
			continue
		}
		action.Choices = append([]string(nil), verified...)
		restricted = append(restricted, action)
	}
	return restricted
}

func nextActionForCheck(kind string, check projectreadiness.Check) projectreadiness.NextAction {
	action := projectreadiness.NextAction{Kind: kind, Executable: projectreadiness.NextActionExecutable(kind)}
	switch kind {
	case projectreadiness.NextActionInstallSupportedRuntime:
		action.RuntimeKind = tstoolchain.NodeCheckKind
		action.Supported = tstoolchain.SupportedNodeMajorsCopy()
		if check.Code == projectreadiness.GapNodeUnsupported {
			action.FoundVersion = check.Version
		}
	case projectreadiness.NextActionRepairRuntimeProbe:
		action.RuntimeKind = tstoolchain.NodeCheckKind
		action.Detail = check.Detail
	case projectreadiness.NextActionPrepareCompiler:
		action.Supported = tstoolchain.SupportedTypescriptVersionsCopy()
		action.FoundVersion = check.FoundVersion
	case projectreadiness.NextActionResolvePackageManager:
		action.PackageManagerKind = check.Kind
		action.FoundVersion = check.FoundVersion
	}
	return action
}
