// Package tssetup runs the consented TypeScript compiler setup a failed
// readiness check offers: it builds the menu of verified installation choices,
// previews the exact frozen command, executes it only after a single explicit
// confirmation, and rechecks readiness afterwards.
package tssetup

import (
	"bufio"
	"context"
	"fmt"
	"io"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// CompilerSetupOfferResult is RunCompilerSetupOffer's outcome: the
// caller-facing policy decision plus enough detail to report it, regardless
// of which of AvailableChoices' kinds was actually selected and
// executed. It intentionally flattens Outcome's own shape (the
// project_package execution path) and PrepareCompilerMiseResult's shape
// (the mise execution path) into one result, so a caller does not have to
// branch on which of the two ran before deciding what to report.
type CompilerSetupOfferResult struct {
	// PolicyRequired is true while readiness.Checks.Policy has not passed,
	// mirroring RunPrepareCompilerMiseSetup's own AC-SET-13 precondition
	// (prepare_compiler.go). The sole production caller
	// (runScanCompilerSetupOffer) only ever reaches this function downstream
	// of a successful loadProjectConfig, so this never actually fires today;
	// the guard exists so that invariant is enforced here too rather than
	// resting entirely on the caller never changing.
	PolicyRequired bool

	// NoChoicesOffered is true either when gapCode's own next action is not
	// the executable prepare-compiler kind (a runtime-boundary gap such as
	// node_missing/node_unsupported), or when readiness's compiler check
	// was not actually failing: AvailableChoices returned an empty
	// menu. Either way, no prompt was ever shown.
	NoChoicesOffered bool

	// Withheld carries AvailableChoices' own reasons for every
	// candidate it ruled out, but only on the NoChoicesOffered branch where
	// a menu was actually built and nothing in it was executable. That is
	// the one outcome where the customer has no next step at all -- the gap
	// line points at --check-project, which re-reports the same gap -- so
	// the reason nothing could be offered is what breaks the loop. It stays
	// nil for a runtime-boundary gap code and for a nil readiness, where no
	// menu was ever computed and there is nothing to explain.
	Withheld []WithheldChoice

	// Cancelled is true when the user selected "cancel", gave an
	// unrecognized or blank answer at either prompt, or declined the
	// single-use confirmation. No setup command ever ran.
	Cancelled bool

	// Choice is the selected menu entry, set once selection completes
	// regardless of what happened afterward. It is the zero ChoiceKind
	// when NoChoicesOffered is true, or when the top-level selection itself
	// was cancelled.
	Choice ChoiceKind

	// Succeeded is true only when the confirmed install actually completed
	// (AC-SET-7): a project_package run via
	// RunConfirmedAndRecheckReadiness, or a mise scope run via
	// installMiseTypescriptProject/Global.
	Succeeded bool

	// FailureDetail is a human-readable cause, populated whenever neither
	// Cancelled nor Succeeded is true: BuildPreview/Execute's own
	// refusal, a non-zero exit or timeout, a mise scope that stopped
	// declaring an installable version between menu construction and
	// confirmation, or a verification failure after an exit-0 mise install.
	FailureDetail string

	// ChangedPaths and ResidueUnknown mirror Outcome's own AC-SET-7
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
// session a real scan's tstoolchain.CompilerUnresolvedError gap offers when a
// controlling terminal is available (AC-SET-9, AC-11): present
// every AvailableChoices entry -- project_package and/or a verified
// mise scope -- in one menu, require an explicit single selection with no
// default, show the resolved preview, require one single-use confirmation,
// run the confirmed choice through its own execution path, and rerun
// readiness (AC-SET-6) only after a successful install.
//
// gapCode is the tstoolchain.CompilerUnresolvedError's own Code, not
// readiness.Checks.Compiler.Code: the two are independent
// projectcheck.Run reads (projectcheck.Run resolves Node and the
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
	menu, projectPackageDirectory, resolutionWithheld := withProjectPackageResolution(AvailableChoices(*readiness), dir, revision, configPath)
	if !menuOffersExecutableChoice(menu) {
		return CompilerSetupOfferResult{NoChoicesOffered: true, Withheld: menu.Withheld}
	}
	if resolutionWithheld != "" {
		// Withholding project_package here removes an entry the customer
		// would otherwise have been offered, unlike the entries
		// AvailableChoices never built: a shorter menu with no
		// explanation reads as "Coach cannot install from my package
		// manager", when the reason is about which directory a single
		// consented command could run in.
		fmt.Fprintf(out, "TypeScript compiler setup: %s is not offered here (%s).\n", ChoiceProjectPackage, resolutionWithheld)
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

	if choice == ChoiceProjectPackage {
		return runProjectPackageSetupOffer(ctx, dir, revision, configPath, projectPackageDirectory, readiness.Checks.PackageManager, out, reader)
	}
	return runMiseSetupOffer(ctx, dir, revision, configPath, choice, out, reader)
}

// menuOffersExecutableChoice reports whether menu carries at least one
// non-cancel choice. AvailableChoices always appends ChoiceCancel
// (choice_menu.go), so len(menu.Choices) == 0 is true only when
// readiness's compiler check is not actually failing; a compiler gap with
// nothing installable still produces a one-entry (cancel-only) menu, which
// must not open an interactive prompt that can only ever be cancelled.
func menuOffersExecutableChoice(menu Menu) bool {
	for _, choice := range menu.Choices {
		if choice.Kind != ChoiceCancel {
			return true
		}
	}
	return false
}
