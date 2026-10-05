package codesignalcli

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// failingReadinessChecks reads checks.Runtime rather than checks.Node: the
// two always carry the same State/Code (see nodeCompatibilityMirror), and
// including both here would double-report every Node gap. checks.PackageManager
// is deliberately excluded here -- its package_manager_* finding is
// setup-scoped (SA-280-045) and handled separately by packageManagerGapEntries,
// not by this table-driven path; including it here as well would double-report
// it.
func failingReadinessChecks(checks projectreadiness.Checks) []projectreadiness.Check {
	var failing []projectreadiness.Check
	for _, check := range []projectreadiness.Check{checks.ProjectShape, checks.Policy, checks.Runtime, checks.Compiler} {
		if check.State == projectreadiness.Fail {
			failing = append(failing, check)
		}
	}
	return failing
}

func readinessFromGapChecks(failing []projectreadiness.Check) ([]projectreadiness.Gap, []projectreadiness.NextAction, projectreadiness.Status) {
	gaps := make([]projectreadiness.Gap, 0, len(failing))
	nextActions := make([]projectreadiness.NextAction, 0, len(failing))
	seenActions := map[string]bool{}
	status := projectreadiness.StatusReady
	for _, check := range failing {
		gaps = append(gaps, projectreadiness.Gap{Code: check.Code})
		status = raiseStatus(status, projectreadiness.StatusForGapCode(check.Code))
		if kind, ok := projectreadiness.NextActionForGapCode(check.Code); ok && !seenActions[kind] {
			seenActions[kind] = true
			nextActions = append(nextActions, nextActionForCheck(kind, check))
		}
	}
	return gaps, nextActions, status
}

func nextActionForCheck(kind string, check projectreadiness.Check) projectreadiness.NextAction {
	action := projectreadiness.NextAction{Kind: kind, Executable: projectreadiness.NextActionExecutable(kind)}
	switch kind {
	case projectreadiness.NextActionInstallSupportedRuntime:
		action.RuntimeKind = readinessNodeCheckKind
		action.Supported = supportedNodeMajorsCopy()
		if check.Code == projectreadiness.GapNodeUnsupported {
			action.FoundVersion = check.Version
		}
	case projectreadiness.NextActionRepairRuntimeProbe:
		action.RuntimeKind = readinessNodeCheckKind
		action.Detail = check.Detail
	case projectreadiness.NextActionPrepareCompiler:
		action.Supported = supportedTypescriptVersionsCopy()
		action.FoundVersion = check.FoundVersion
	case projectreadiness.NextActionResolvePackageManager:
		action.PackageManagerKind = check.Kind
		action.FoundVersion = check.FoundVersion
	}
	return action
}

func compilerDeclarationWarnings(check projectreadiness.Check) []projectreadiness.Warning {
	if check.Code != projectreadiness.WarnCompilerDeclarationMismatch {
		return nil
	}
	warnings := make([]projectreadiness.Warning, 0, len(check.DeclarationMismatches))
	for _, mismatch := range check.DeclarationMismatches {
		warnings = append(warnings, projectreadiness.Warning{
			Code:              projectreadiness.WarnCompilerDeclarationMismatch,
			DeclaredVersion:   mismatch.Declared,
			FoundVersion:      check.Version,
			DeclarationOrigin: check.DeclarationOrigin,
			Root:              mismatch.Root,
		})
	}
	return warnings
}

func raiseStatus(status, candidate projectreadiness.Status) projectreadiness.Status {
	if projectreadiness.StatusRank(candidate) > projectreadiness.StatusRank(status) {
		return candidate
	}
	return status
}

// appendPackageManagerFinding's foundVersion is set only for the project
// adapter's own finding: projectreadiness.MiseChoice carries no version, since a
// rejected mise scope is unverifiable before any version is ever read.
func appendPackageManagerFinding(gaps []projectreadiness.Gap, actions []projectreadiness.NextAction, status projectreadiness.Status, code, kind, foundVersion string) ([]projectreadiness.Gap, []projectreadiness.NextAction, projectreadiness.Status) {
	gaps = append(gaps, projectreadiness.Gap{Code: code, PackageManagerKind: kind})
	if actionKind, ok := projectreadiness.NextActionForGapCode(code); ok {
		actions = append(actions, projectreadiness.NextAction{Kind: actionKind, Executable: projectreadiness.NextActionExecutable(actionKind), PackageManagerKind: kind, FoundVersion: foundVersion})
	}
	status = raiseStatus(status, projectreadiness.StatusForGapCode(code))
	return gaps, actions, status
}
