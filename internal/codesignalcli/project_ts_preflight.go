package codesignalcli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/prompt"
)

// ProjectConfigErrorWithReadiness enriches a projectconfig.ConfigError with the
// full TypeScript project-readiness snapshot computed for the same
// dir/revision/configPath. prepareProjectAnalysis's loadProjectConfig short
// circuit means only the policy failure would otherwise ever reach
// classifyAnalysisError, masking a simultaneous compiler gap (AC-SET-13).
// Unwrap returns the original *projectconfig.ConfigError so
// errors.As(err, &plainTarget) still matches through this wrapper exactly as
// it did before wrapping existed.
type ProjectConfigErrorWithReadiness struct {
	*projectconfig.ConfigError
	Readiness  *projectreadiness.Result
	ConfigPath string
}

func (e *ProjectConfigErrorWithReadiness) Unwrap() error { return e.ConfigError }

// CompilerUnresolvedErrorWithReadiness enriches a CompilerUnresolvedError
// with the full TypeScript project-readiness snapshot computed for the same
// dir/revision/configPath, so a real scan's controlling-terminal branch can
// drive the interactive compiler-setup offer from the same facts the gap
// itself was raised from, without a second, possibly racing readiness
// computation. Unwrap returns the original
// *CompilerUnresolvedError so errors.As(err, &plainTarget) still matches
// through this wrapper exactly as it did before wrapping existed -- in
// particular, classifyAnalysisError's own no-controlling-terminal fallback
// needs no change at all.
// ConfigPath is deliberately absent: *CompilerUnresolvedError already
// carries the policy path RemediationLine() renders, and a second copy here
// could drift from it, pointing the printed remediation and the setup
// session it precedes at two different policies.
type CompilerUnresolvedErrorWithReadiness struct {
	*CompilerUnresolvedError
	Readiness *projectreadiness.Result
	Revision  string
}

func (e *CompilerUnresolvedErrorWithReadiness) Unwrap() error { return e.CompilerUnresolvedError }

func alsoFailingGapLine(code, configPath string) string {
	return code + ": also failing, run " + typescriptInvocation("--check-project", configPath)
}

// onATerminal qualifies a command that refuses without a controlling
// terminal. Both commands Coach names as remediation are interactive, and
// this line is printed exactly where no terminal was available -- so a
// piped caller that runs either one verbatim gets exit 2 and no progress.
// Saying so costs a clause; discovering it costs an invocation.
func onATerminal(invocation string) string {
	return "on a terminal: " + invocation
}

// gapCodeIsExecutablePrepareCompiler reports whether gapCode's next action,
// per the authoritative projectreadiness.KnownGapCodes(), is the executable prepare-compiler
// kind -- the only kind Coach can actually run a command for
// (projectreadiness.NextActionExecutable).
func gapCodeIsExecutablePrepareCompiler(gapCode string) bool {
	kind, ok := projectreadiness.NextActionForGapCode(gapCode)
	return ok && projectreadiness.NextActionExecutable(kind)
}

// CompilerSetupOfferResult is RunCompilerSetupOffer's outcome: the
// caller-facing policy decision plus enough detail to report it, regardless
// of which of AvailableSetupChoices' kinds was actually selected and
// executed. It intentionally flattens SetupOutcome's own shape (the
// project_package execution path) and PrepareCompilerMiseResult's shape
// (the mise execution path) into one result, so a caller does not have to
// branch on which of the two ran before deciding what to report.
type CompilerSetupOfferResult struct {
	// PolicyRequired is true while readiness.Checks.Policy has not passed,
	// mirroring RunPrepareCompilerMiseSetup's own AC-SET-13 precondition
	// (project_ts_compiler_mise_install.go). The sole production caller
	// (runScanCompilerSetupOffer) only ever reaches this function downstream
	// of a successful loadProjectConfig, so this never actually fires today;
	// the guard exists so that invariant is enforced here too rather than
	// resting entirely on the caller never changing.
	PolicyRequired bool

	// NoChoicesOffered is true either when gapCode's own next action is not
	// the executable prepare-compiler kind (a runtime-boundary gap such as
	// node_missing/node_unsupported), or when readiness's compiler check
	// was not actually failing: AvailableSetupChoices returned an empty
	// menu. Either way, no prompt was ever shown.
	NoChoicesOffered bool

	// Withheld carries AvailableSetupChoices' own reasons for every
	// candidate it ruled out, but only on the NoChoicesOffered branch where
	// a menu was actually built and nothing in it was executable. That is
	// the one outcome where the customer has no next step at all -- the gap
	// line points at --check-project, which re-reports the same gap -- so
	// the reason nothing could be offered is what breaks the loop. It stays
	// nil for a runtime-boundary gap code and for a nil readiness, where no
	// menu was ever computed and there is nothing to explain.
	Withheld []WithheldSetupChoice

	// Cancelled is true when the user selected "cancel", gave an
	// unrecognized or blank answer at either prompt, or declined the
	// single-use confirmation. No setup command ever ran.
	Cancelled bool

	// Choice is the selected menu entry, set once selection completes
	// regardless of what happened afterward. It is the zero SetupChoiceKind
	// when NoChoicesOffered is true, or when the top-level selection itself
	// was cancelled.
	Choice SetupChoiceKind

	// Succeeded is true only when the confirmed install actually completed
	// (AC-SET-7): a project_package run via
	// RunConfirmedSetupAndRecheckReadiness, or a mise scope run via
	// installMiseTypescriptProject/Global.
	Succeeded bool

	// FailureDetail is a human-readable cause, populated whenever neither
	// Cancelled nor Succeeded is true: BuildSetupPreview/ExecuteSetup's own
	// refusal, a non-zero exit or timeout, a mise scope that stopped
	// declaring an installable version between menu construction and
	// confirmation, or a verification failure after an exit-0 mise install.
	FailureDetail string

	// ChangedPaths and ResidueUnknown mirror SetupOutcome's own AC-SET-7
	// disclosure for a failed project_package run; both stay nil/false for
	// every other path (mise's own installer produces no such disclosure,
	// and a cancelled or successful run has nothing to disclose).
	ChangedPaths   []string
	ResidueUnknown bool

	// PostInstallReadiness is populated only after a successful install
	// (AC-SET-6): the complete readiness rerun a caller must consult before
	// deciding whether the scan that offered this setup may continue
	// (AC-7).
	PostInstallReadiness *projectreadiness.Result

	// RuntimeGapCode is set when readiness.Checks.Runtime independently fails
	// with a gap this offer has no setup command for (node_missing,
	// node_unsupported). A runtime-boundary gap takes priority over a
	// simultaneously offerable prepare_compiler action, matching
	// RunPrepareCompilerMiseSetup. No prompt is shown when this is set.
	RuntimeGapCode string
}

// RunCompilerSetupOffer runs the interactive, combined compiler-setup
// session a real scan's CompilerUnresolvedError gap offers when a
// controlling terminal is available (AC-SET-9, AC-11): present
// every AvailableSetupChoices entry -- project_package and/or a verified
// mise scope -- in one menu, require an explicit single selection with no
// default, show the resolved preview, require one single-use confirmation,
// run the confirmed choice through its own execution path, and rerun
// readiness (AC-SET-6) only after a successful install.
//
// gapCode is the CompilerUnresolvedError's own Code, not
// readiness.Checks.Compiler.Code: the two are independent
// CheckProjectReadiness reads (project_readiness.go resolves Node and the
// compiler separately), so a runtime-boundary gap (node_missing,
// node_unsupported) can coexist with a readiness snapshot whose compiler
// check independently offers an installable menu entry. A blocking runtime
// check is gated first (readinessHasBlockingRuntimeGap), then
// gapCodeIsExecutablePrepareCompiler, matching RunPrepareCompilerMiseSetup
// so the scan offer and the standalone flag cannot disagree (AC-SET-10).
//
// Every prompt is read from in and every line of output written to out at
// most once per session: selection and confirmation are each read exactly
// once into a local variable that is used exactly once and then goes out of
// scope, and nothing here loops back to prompt again afterward -- so an
// answer already consumed can never be replayed against a second execution
// within the same call, and a leftover, unread answer left on in can never
// trigger one either.
//
// readiness.Checks.Policy must already be projectreadiness.Pass, mirroring
// RunPrepareCompilerMiseSetup's own precondition: a reviewed, committed
// policy is required before compiler setup is ever offered (AC-SET-13).
func RunCompilerSetupOffer(ctx context.Context, dir, revision, configPath, gapCode string, readiness *projectreadiness.Result, in io.Reader, out io.Writer) CompilerSetupOfferResult {
	if readiness != nil && readiness.Checks.Policy.State != projectreadiness.Pass {
		return CompilerSetupOfferResult{PolicyRequired: true}
	}
	if code, blocking := readinessHasBlockingRuntimeGap(readiness); blocking {
		return CompilerSetupOfferResult{RuntimeGapCode: code}
	}
	if !gapCodeIsExecutablePrepareCompiler(gapCode) {
		return CompilerSetupOfferResult{NoChoicesOffered: true}
	}
	if readiness == nil {
		return CompilerSetupOfferResult{NoChoicesOffered: true}
	}
	menu, projectPackageDirectory, resolutionWithheld := withProjectPackageResolution(AvailableSetupChoices(*readiness), dir, revision, configPath)
	if !menuOffersExecutableChoice(menu) {
		return CompilerSetupOfferResult{NoChoicesOffered: true, Withheld: menu.Withheld}
	}
	if resolutionWithheld != "" {
		// Withholding project_package here removes an entry the customer
		// would otherwise have been offered, unlike the entries
		// AvailableSetupChoices never built: a shorter menu with no
		// explanation reads as "Coach cannot install from my package
		// manager", when the reason is about which directory a single
		// consented command could run in.
		fmt.Fprintf(out, "TypeScript compiler setup: %s is not offered here (%s).\n", SetupChoiceProjectPackage, resolutionWithheld)
	}

	// The diagnosis precedes the menu: a list of installation choices is not
	// an explanation, and a customer asked to authorize a network install
	// deserves to know which check failed and what happens afterward before
	// answering, not after. It names the gap code rather than repeating the
	// caller's own "run --check-project" remediation line, which would be
	// stale advice the moment the install succeeds and the scan resumes.
	fmt.Fprintf(out, "TypeScript compiler setup: this scan cannot resolve a supported TypeScript compiler (%s).\n", gapCode)
	fmt.Fprintln(out, "TypeScript compiler setup: if you confirm a choice below and it succeeds, Coach rechecks readiness and resumes this same scan only if that recheck reports no gap; otherwise no report is produced.")

	reader := bufio.NewReader(in)
	choice, cancelled := promptForCompilerSetupChoice(out, reader, menu.Choices)
	if cancelled {
		return CompilerSetupOfferResult{Cancelled: true}
	}

	if choice == SetupChoiceProjectPackage {
		return runProjectPackageSetupOffer(ctx, dir, revision, configPath, projectPackageDirectory, readiness.Checks.PackageManager, out, reader)
	}
	return runMiseSetupOffer(ctx, dir, revision, configPath, choice, out, reader)
}

// menuOffersExecutableMiseChoice narrows menuOffersExecutableChoice to the
// kinds the interim --prepare-compiler flag can actually run. The scan's own
// offer (RunCompilerSetupOffer) executes every kind and so uses the wider
// predicate; only a printed --prepare-compiler command needs this one.
func menuOffersExecutableMiseChoice(menu SetupChoiceMenu) bool {
	for _, choice := range menu.Choices {
		if choice.Kind == SetupChoiceProjectMise || choice.Kind == SetupChoiceGlobalMise {
			return true
		}
	}
	return false
}

// promptForSetupConfirmation is the single-use explicit confirmation gate
// for a project_package setup command: only the exact token "confirm"
// (case-insensitive) proceeds, read exactly once from reader -- there is no
// retry, mirroring promptForMiseInstallConfirmation's own gate.
func promptForSetupConfirmation(out io.Writer, reader *bufio.Reader) bool {
	fmt.Fprintln(out, "Type 'confirm' to run this setup command now, or anything else to cancel without making any change:")
	fmt.Fprint(out, "> ")
	answer, _ := prompt.ReadLine(reader)
	return strings.EqualFold(strings.TrimSpace(answer), "confirm")
}

// Defensive: packageManagerContexts falls back to the worktree root,
// so it does not return an empty slice today.
