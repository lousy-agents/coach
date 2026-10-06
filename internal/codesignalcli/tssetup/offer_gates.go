package tssetup

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// readinessHasBlockingRuntimeGap reports whether readiness.Checks.Runtime
// independently fails with a gap code that is not the executable
// prepare-compiler kind, through the same gapCodeIsExecutablePrepareCompiler
// predicate the real scan's own compiler-setup gate uses
// (offer_gates.go). readinessFromGapChecks (projectcheck/
// aggregate.go) derives Runtime's and Compiler's next actions independently
// of one another, so a failing runtime check can coexist in NextActions with
// a genuinely executable prepare_compiler entry; without this check,
// RunPrepareCompilerMiseSetup would offer to install a compiler while the
// host Node runtime that would run it is still missing or unsupported --
// exactly the gap a real scan's own PrepareTSRuntime never reaches, since it
// resolves host Node first and fails fast there.
func readinessHasBlockingRuntimeGap(readiness *projectreadiness.Result) (string, bool) {
	if readiness == nil {
		return "", false
	}
	code := readiness.Checks.Runtime.Code
	if readiness.Checks.Runtime.State != projectreadiness.Fail || code == "" {
		return "", false
	}
	if gapCodeIsExecutablePrepareCompiler(code) {
		return "", false
	}
	return code, true
}

// gapCodeIsExecutablePrepareCompiler reports whether gapCode's next action,
// per the authoritative projectreadiness.KnownGapCodes(), is the executable prepare-compiler
// kind -- the only kind Coach can actually run a command for
// (projectreadiness.NextActionExecutable).
func gapCodeIsExecutablePrepareCompiler(gapCode string) bool {
	kind, ok := projectreadiness.NextActionForGapCode(gapCode)
	return ok && projectreadiness.NextActionExecutable(kind)
}

func menuOffersChoice(menu Menu, kind ChoiceKind) bool {
	for _, choice := range menu.Choices {
		if choice.Kind == kind {
			return true
		}
	}
	return false
}
