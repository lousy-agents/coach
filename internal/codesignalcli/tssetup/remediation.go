package tssetup

import (
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// onATerminal qualifies a command that refuses without a controlling
// terminal. Both commands Coach names as remediation are interactive, and
// this line is printed exactly where no terminal was available -- so a
// piped caller that runs either one verbatim gets exit 2 and no progress.
// Saying so costs a clause; discovering it costs an invocation.
func onATerminal(invocation string) string {
	return "on a terminal: " + invocation
}

// PrepareCompilerRemediationWithReadiness extends PrepareCompilerRemediation
// for a gap already wrapped with its own readiness snapshot
// (CompilerUnresolvedErrorWithReadiness): gapCodeIsExecutablePrepareCompiler
// alone is not enough to promise the printed command will do anything, since
// AvailableChoices can still resolve to a menu --prepare-compiler cannot
// act on. The test is what that flag would execute, not what the scan's own
// combined menu offers: RunPrepareCompilerMiseSetup discards every non-mise
// kind (filterMiseChoiceKinds, project_ts_compiler_mise_install.go), so a
// menu whose only executable entry is project_package -- an npm/pnpm/Bun
// repository that has simply never installed its declared compiler, and the
// scan's own controlling-terminal offer resolves it -- is as much a dead end
// as an empty one. Printing the command in either case would open a session,
// offer nothing, and exit 0 having changed nothing while the compiler is
// still missing -- exactly the dead end PrepareCompilerRemediation's own doc
// comment already promises never to name, and the one shape a piped CI
// operator cannot tell apart from success. readiness == nil falls back to
// PrepareCompilerRemediation's own gapCode-only decision, matching
// WrapCompilerUnresolvedErrorWithReadiness's contract of returning the
// original error unchanged when readiness itself could not be computed.
func PrepareCompilerRemediationWithReadiness(gapCode, configPath string, readiness *projectreadiness.Result) string {
	if readiness == nil {
		return PrepareCompilerRemediation(gapCode, configPath)
	}
	if !gapCodeIsExecutablePrepareCompiler(gapCode) {
		return ""
	}
	if !menuOffersExecutableMiseChoice(AvailableChoices(*readiness)) {
		return ""
	}
	return onATerminal(typescriptInvocation("--prepare-compiler", configPath))
}

// PrepareCompilerRemediation names the interactive, consented mise
// TypeScript compiler-setup command that resolves a tstoolchain.CompilerUnresolvedError
// gap, for AC-SET-9's appended no-controlling-terminal remediation line. It
// returns "" for a gap code whose next action is not the executable
// prepare-compiler kind (e.g. node_missing, node_unsupported): Coach has no
// setup command that fixes a runtime-boundary gap, so appending one would
// name a command that either does nothing or targets the wrong problem.
func PrepareCompilerRemediation(gapCode, configPath string) string {
	if !gapCodeIsExecutablePrepareCompiler(gapCode) {
		return ""
	}
	return onATerminal(typescriptInvocation("--prepare-compiler", configPath))
}

// ScanSetupOfferRemediation names the interactive scan invocation itself
// (R2) when a compiler gap's only genuinely executable setup choice is
// project_package: --prepare-compiler can never resolve that choice, since
// RunPrepareCompilerMiseSetup discards every non-mise kind
// (project_ts_compiler_mise_install.go), so
// PrepareCompilerRemediationWithReadiness withholds its own command for
// exactly this menu. Without this, a no-TTY invocation whose only path
// forward is project_package was told nothing beyond the bare
// --check-project line, even though rerunning the same scan on a terminal
// would open the combined setup offer (RunCompilerSetupOffer) and resolve
// it. It returns "" whenever PrepareCompilerRemediationWithReadiness would
// already offer its own command (a verified mise choice exists too), so the
// two remediations are never both printed for the same gap.
func ScanSetupOfferRemediation(gapCode, configPath string, readiness *projectreadiness.Result) string {
	if readiness == nil || !gapCodeIsExecutablePrepareCompiler(gapCode) {
		return ""
	}
	menu := AvailableChoices(*readiness)
	if !menuOffersExecutableChoice(menu) || menuOffersExecutableMiseChoice(menu) {
		return ""
	}
	return onATerminal(typescriptScanInvocation(configPath) + " -- offers project-package setup")
}

// typescriptScanInvocation names the bare `coach codesignal --baseline`
// scan itself, distinct from typescriptInvocation's own --check-project/
// --prepare-compiler forms: R2's remediation points at rerunning the
// original scan on a terminal, not at a standalone subcommand.
func typescriptScanInvocation(configPath string) string {
	invocation := "coach codesignal --baseline"
	if configPath != "" {
		invocation += " --project-config " + configPath
	}
	return invocation + " --project-language typescript"
}

func typescriptInvocation(flag, projectConfigPath string) string {
	invocation := "coach codesignal --baseline " + flag + " --project-language typescript"
	if projectConfigPath != "" {
		invocation += " --project-config " + projectConfigPath
	}
	return invocation
}
