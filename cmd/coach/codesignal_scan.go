// Scan orchestration for `coach codesignal`: what one scan attempt does with
// the error it returns, which interactive offer owns that gap, and what a
// customer is told when no offer can. main.go keeps process entry, flag
// parsing, and the mode dispatch that routes here.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

// nonInteractiveRequested reports whether f's own --no-interactive flag or a
// non-empty CI environment variable requests that a scan treat itself as
// non-interactive, even when a real controlling terminal is attached. CI
// systems conventionally set CI without anyone passing a flag (GitHub
// Actions, GitLab CI, Jenkins, CircleCI, ...), and a pty-allocating CI job
// still reports a genuine controlling terminal (docker run -t, ssh -t,
// script -qec) -- honoring CI here, not only the flag, is what actually
// prevents an unattended interactive offer from hanging such a job forever.
func nonInteractiveRequested(f codesignalFlags) bool {
	return f.noInteractive || os.Getenv("CI") != ""
}

// interactiveRefusalReason names why a command that can only work by
// prompting must refuse, or "" when it may proceed. The two halves are
// reported apart because they are not the same situation: a piped invocation
// has nobody to ask, while --no-interactive or a non-empty CI means a real
// terminal is attached and Coach was asked not to use it.
//
// The absent terminal is reported first when both hold, because it is the
// fact the customer can do something about: on a CI runner CI is set for
// every job, so telling a genuinely piped invocation that it was declined
// would point at an environment variable whose removal changes nothing --
// there is still no terminal to prompt on.
func interactiveRefusalReason(f codesignalFlags, stdin *os.File) string {
	if !codesignalcli.HasControllingTerminal(stdin) {
		return "no controlling terminal is available"
	}
	if nonInteractiveRequested(f) {
		return "this invocation is non-interactive (--no-interactive, or a non-empty CI environment variable)"
	}
	return ""
}

type scanOfferKind string

const scanOfferCompilerSetup scanOfferKind = "compiler_setup"

type scanOfferBudget map[scanOfferKind]bool

func newScanOfferBudget() scanOfferBudget {
	return scanOfferBudget{scanOfferCompilerSetup: true}
}

func (b scanOfferBudget) allows(kind scanOfferKind) bool {
	return b[kind]
}

func (b scanOfferBudget) without(kind scanOfferKind) scanOfferBudget {
	next := make(scanOfferBudget, len(b))
	for k, v := range b {
		next[k] = v
	}
	next[kind] = false
	return next
}

type optionalPreparationResult struct {
	Declined  bool
	Cancelled bool
	Succeeded bool
	Failed    bool
}

func shouldRenderAfterOptionalPreparation(result optionalPreparationResult) bool {
	return !result.Cancelled && !result.Failed
}

var runOptionalScanPreparation = func(dir string, f codesignalFlags, stdout, stderr *os.File) optionalPreparationResult {
	return optionalPreparationResult{}
}

func interruptibleContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func runCodesignalScan(dir string, f codesignalFlags, stdout, stderr *os.File, budget scanOfferBudget) int {
	report, projectExitCode, err := runOneScan(dir, f, stderr)
	if err != nil {
		noInteractive := nonInteractiveRequested(f)
		if scanShouldAuthorProjectConfig(err, f.projectLanguage, f.projectConfig, noInteractive) {
			return runScanProjectConfigAuthoring(dir, f, err, stdout, stderr)
		}
		if wrapped, ok := scanShouldOfferCompilerSetup(err, noInteractive); budget.allows(scanOfferCompilerSetup) && ok {
			return runScanCompilerSetupOffer(dir, f, stdout, stderr, wrapped, budget)
		}
		return classifyAnalysisError(err, f.projectLanguage, noInteractive, stderr)
	}
	if result := runOptionalScanPreparation(dir, f, stdout, stderr); !shouldRenderAfterOptionalPreparation(result) {
		return 2
	}
	return renderScanResult(report, projectExitCode, f.format, stdout, stderr)
}

func runOneScan(dir string, f codesignalFlags, stderr *os.File) (*codesignal.Report, int, error) {
	if f.baseline {
		return runBaselineAnalysis(dir, f, stderr)
	}
	return runDiffAnalysis(dir, f, stderr)
}

func wrapScanAnalysisError(err error, dir, revision, configPath string, stderr *os.File) error {
	var unresolved *codesignalcli.CompilerUnresolvedError
	if errors.As(err, &unresolved) {
		return codesignalcli.WrapCompilerUnresolvedErrorWithReadiness(unresolved, dir, revision, configPath)
	}
	var runtimeErr *codesignalcli.RuntimeUnresolvedError
	if errors.As(err, &runtimeErr) {
		return runtimeErr
	}
	fmt.Fprintf(stderr, "coach codesignal: analysis failed: %s\n", err)
	return nil
}

func renderScanResult(report *codesignal.Report, projectExitCode int, format string, stdout, stderr *os.File) int {
	if report == nil {
		if projectExitCode != 0 {
			return projectExitCode
		}
		return 1
	}
	if exitCode := renderReport(report, format, stdout, stderr); exitCode != 0 {
		return exitCode
	}
	return projectExitCode
}

// withheldSetupChoicesLine renders AvailableSetupChoices' own reasons for
// ruling out every candidate, for the one outcome that leaves the customer
// without a next step: a real compiler gap where nothing at all could be
// offered. The gap line printed above it names --check-project, which only
// re-reports the same gap, so without the reasons (an unconfigured mise, a
// manifest that declares no supported compiler, a rejected package manager)
// there is nothing to act on. It returns "" when no menu was built, so the
// runtime-boundary path's output is unchanged.
func withheldSetupChoicesLine(withheld []codesignalcli.WithheldSetupChoice) string {
	if len(withheld) == 0 {
		return ""
	}
	reasons := make([]string, 0, len(withheld))
	for _, entry := range withheld {
		reasons = append(reasons, fmt.Sprintf("%s (%s)", entry.Kind, entry.Reason))
	}
	return "coach codesignal: no compiler-setup choice is executable here: " + strings.Join(reasons, ", ") + "."
}

// setupResidueDisclosure renders AC-SET-7's "identify files that may have
// changed" for a failed setup. ResidueUnknown is not a quieter version of an
// empty ChangedPaths: it means Coach could not read what changed at all, and
// the fallback path it carries is the working directory itself -- which
// renders repository-relative as a bare ".", indistinguishable from a precise
// finding. Saying so plainly is the difference between "nothing changed" and
// "Coach does not know", which is exactly the distinction SetupOutcome's own
// contract asks callers to preserve.
func setupResidueDisclosure(result codesignalcli.CompilerSetupOfferResult) string {
	if result.ResidueUnknown {
		if len(result.ChangedPaths) > 0 {
			return "coach codesignal: Coach could not determine which files the setup command changed under " + strings.Join(result.ChangedPaths, ", ") + "; inspect it before rerunning."
		}
		return "coach codesignal: Coach could not determine which files the setup command changed; inspect the working tree before rerunning."
	}
	if len(result.ChangedPaths) == 0 {
		return ""
	}
	return "coach codesignal: the setup command may have changed: " + strings.Join(result.ChangedPaths, ", ")
}

// scanShouldOfferCompilerSetup reports whether err is a scan-time
// CompilerUnresolvedError carrying its own readiness snapshot (wrapped by
// runBaselineAnalysis/runDiffAnalysis for AC-SET-9's controlling-terminal
// branch), noInteractive is false, and a controlling terminal is available
// to run the interactive offer on. With noInteractive true or no controlling
// terminal, classifyAnalysisError's existing message-only path (unwrapping
// straight through to the plain *CompilerUnresolvedError) is unchanged --
// this is what keeps a pty-allocating but genuinely unattended invocation
// (R1) from opening a prompt nobody will ever answer.
func scanShouldOfferCompilerSetup(err error, noInteractive bool) (*codesignalcli.CompilerUnresolvedErrorWithReadiness, bool) {
	var wrapped *codesignalcli.CompilerUnresolvedErrorWithReadiness
	if !errors.As(err, &wrapped) {
		return nil, false
	}
	if noInteractive || !codesignalcli.HasControllingTerminal(os.Stdin) {
		return nil, false
	}
	return wrapped, true
}

// runScanCompilerSetupOffer implements the interactive compiler-setup offer
// for a real scan's CompilerUnresolvedError gap (AC-SET-9): it consumes the
// readiness snapshot already computed alongside the gap (wrapped.Readiness)
// rather than recomputing it a third time, runs the combined setup menu,
// and -- only when the confirmed install succeeds and its AC-SET-6
// readiness rerun reports ready or ready_with_limits --
// retries the same scan once more (compiler-setup consumed from the offer
// budget: AC-SET-9's offer runs at most once per invocation), so the customer's original
// invocation still produces a report. Every other outcome (no choices
// offered, cancelled, failed, or a rerun that still reports a gap) exits 2
// with no CodeSignal report ever rendered (AC-18); wrapped.RemediationLine()
// is printed for every one of those outcomes, but withheld on the
// succeeded-and-continued path, where the gap it names no longer exists.
// The gap the offer exists for is named before the menu opens rather than
// here -- RunCompilerSetupOffer is where it is known that a menu will
// actually open, and diagnosing it here would also promise choices to a
// customer about to be told there are none.
func runScanCompilerSetupOffer(dir string, f codesignalFlags, stdout, stderr *os.File, wrapped *codesignalcli.CompilerUnresolvedErrorWithReadiness, budget scanOfferBudget) int {
	ctx, stop := interruptibleContext()
	defer stop()
	result := codesignalcli.RunCompilerSetupOffer(ctx, dir, wrapped.Revision, wrapped.ConfigPath, wrapped.Code, wrapped.Readiness, os.Stdin, stderr)

	if shouldContinueAfterSetup(result) {
		return runCodesignalScan(dir, f, stdout, stderr, budget.without(scanOfferCompilerSetup))
	}

	if result.PolicyRequired {
		fmt.Fprintln(stderr, wrapped.RemediationLine())
		fmt.Fprintln(stderr, "coach codesignal: a reviewed, committed policy is required before compiler setup; run guided policy authoring first (author_policy).")
		return 2
	}

	if result.RuntimeGapCode != "" {
		fmt.Fprintln(stderr, wrapped.RemediationLine())
		fmt.Fprintf(stderr, "coach codesignal: %s is a runtime-boundary gap; Coach has no compiler-setup command for it.\n", result.RuntimeGapCode)
		return 2
	}

	fmt.Fprintln(stderr, wrapped.RemediationLine())

	if result.NoChoicesOffered {
		if line := withheldSetupChoicesLine(result.Withheld); line != "" {
			fmt.Fprintln(stderr, line)
		}
		return 2
	}
	if result.Cancelled {
		fmt.Fprintln(stderr, "coach codesignal: compiler setup was cancelled or not confirmed; no report was produced and no setup command ran.")
		return 2
	}
	if !result.Succeeded {
		fmt.Fprintf(stderr, "coach codesignal: compiler setup failed (%s); no report was produced.\n", result.FailureDetail)
		if line := setupResidueDisclosure(result); line != "" {
			fmt.Fprintln(stderr, line)
		}
		return 2
	}
	if result.PostInstallReadiness == nil {
		fmt.Fprintln(stderr, "coach codesignal: compiler setup succeeded, but the readiness recheck itself failed to run; no report was produced.")
		return 2
	}
	fmt.Fprintln(stderr, "coach codesignal: compiler setup succeeded, but the readiness recheck still reports a gap; no report was produced.")
	return 2
}

func readinessAllowsScan(status codesignalcli.ReadinessStatus) bool {
	return status == codesignalcli.StatusReady || status == codesignalcli.StatusReadyWithLimits
}

// shouldContinueAfterSetup is AC-SET-6/AC-7's continuation gate: the same
// scan may resume after a consented compiler-setup action only when that
// action actually completed (Succeeded) and its own mandatory readiness
// rerun (PostInstallReadiness) reports no gap (readinessAllowsScan). A
// successful install whose rerun still reports needs_prerequisite/
// needs_policy/outside_support must still exit 2, not resume: the install
// ran, but it did not resolve what the scan needed.
func shouldContinueAfterSetup(result codesignalcli.CompilerSetupOfferResult) bool {
	return result.Succeeded && result.PostInstallReadiness != nil && readinessAllowsScan(result.PostInstallReadiness.Status)
}

// classifyAnalysisError never sees a project_backend_unavailable (exit 3)
// error -- prepareProjectAnalysis handles that case separately by returning
// a diagnostic instead of an error. The base ProjectConfigError message
// stays unconditional on --project-language: loadProjectConfig runs before
// resolveProjectBackend and never receives --project-language (see
// prepareProjectAnalysis), so a class-2 config failure's own message is
// language-independent by construction
// (project_contract_acceptance_test.go guards this invariant) and must stay
// that way. AC-SET-9's appended --suggest-project-config remediation IS
// language-scoped (AC-2: an offered command must actually be supported for
// the language it is offered to; SuggestProjectConfigRemediation), and so is
// ProjectConfigErrorWithReadiness's extra AC-SET-13 gap-report line:
// prepareProjectAnalysis only attaches readiness for --project-language
// typescript, since readiness itself is TypeScript-specific.
//
// noInteractive forces hasControllingTerminal to false even on a real
// controlling terminal: this is the same shape a piped/no-TTY invocation
// already produces, which is what makes falling through here for a
// non-interactive scan byte-identical to that shape rather than a third,
// bespoke one. The CompilerUnresolvedErrorWithReadiness branch (checked
// before the plain CompilerUnresolvedError case, since Unwrap would
// otherwise match the latter first) additionally withholds the appended
// --prepare-compiler command whenever the readiness snapshot's own
// AvailableSetupChoices menu would offer nothing (PrepareCompilerRemediationWithReadiness),
// so a customer is never handed a command that opens only to report it had
// nothing to do. When that withholding fires because the menu's only
// genuinely executable choice is project_package -- a choice
// --prepare-compiler can never run -- ScanSetupOfferRemediation (R2) names
// rerunning this same scan on a terminal instead, so that repository is
// still told a next step rather than only the bare --check-project line.
func classifyAnalysisError(err error, language string, noInteractive bool, stderr *os.File) int {
	report := analysisErrorReportFor(err, language, codesignalcli.HasControllingTerminal(os.Stdin) && !noInteractive)
	for _, line := range report.lines {
		fmt.Fprintln(stderr, line)
	}
	return report.exitCode
}

// analysisErrorReport is what one classified analysis error produces: the
// stderr lines in order, and the exit code. Separating the classification
// from the writing is what lets each error class stay a single expression
// and the shared class-2 shape below exist at all.
type analysisErrorReport struct {
	lines    []string
	exitCode int
}

// analysisErrorReportFor matches err against the error classes in the order
// their wrappers require: each *WithReadiness wrapper comes before the type
// its Unwrap would otherwise satisfy first.
func analysisErrorReportFor(err error, language string, hasControllingTerminal bool) analysisErrorReport {
	var unresolvedWithReadiness *codesignalcli.CompilerUnresolvedErrorWithReadiness
	if errors.As(err, &unresolvedWithReadiness) {
		remediation := codesignalcli.PrepareCompilerRemediationWithReadiness(unresolvedWithReadiness.Code, unresolvedWithReadiness.ConfigPath, unresolvedWithReadiness.Readiness)
		if remediation == "" {
			remediation = codesignalcli.ScanSetupOfferRemediation(unresolvedWithReadiness.Code, unresolvedWithReadiness.ConfigPath, unresolvedWithReadiness.Readiness)
		}
		return classTwoReport(unresolvedWithReadiness.RemediationLine(), nil,
			codesignalcli.AppendedRemediationLine(hasControllingTerminal, language, remediation))
	}
	var unresolved *codesignalcli.CompilerUnresolvedError
	if errors.As(err, &unresolved) {
		return classTwoReport(unresolved.RemediationLine(), nil,
			codesignalcli.AppendedRemediationLine(hasControllingTerminal, language, codesignalcli.PrepareCompilerRemediation(unresolved.Code, unresolved.ConfigPath)))
	}
	var runtimeErr *codesignalcli.RuntimeUnresolvedError
	if errors.As(err, &runtimeErr) {
		return classTwoReport(runtimeErr.RemediationLine(), nil, "")
	}
	var configErrWithReadiness *codesignalcli.ProjectConfigErrorWithReadiness
	if errors.As(err, &configErrWithReadiness) {
		return classTwoReport(configErrWithReadiness.Message,
			codesignalcli.AlsoFailingGapLines(configErrWithReadiness.Readiness, configErrWithReadiness.ConfigPath),
			codesignalcli.AppendedRemediationLine(hasControllingTerminal, language, codesignalcli.SuggestProjectConfigRemediation(language)))
	}
	var configErr *codesignalcli.ProjectConfigError
	if errors.As(err, &configErr) {
		return classTwoReport(configErr.Message, nil,
			codesignalcli.AppendedRemediationLine(hasControllingTerminal, language, codesignalcli.SuggestProjectConfigRemediation(language)))
	}
	var opErr *codesignalcli.OperationalError
	if errors.As(err, &opErr) {
		return analysisErrorReport{lines: []string{opErr.Message}, exitCode: 1}
	}
	return analysisErrorReport{lines: []string{err.Error()}, exitCode: 1}
}

// classTwoReport assembles the shape every class-2 diagnostic shares (owner
// decision D3): the gap's own message, then any further gaps AC-SET-13
// requires reporting alongside it, then AC-SET-9's appended remediation when
// one is offered. An unoffered remediation is dropped rather than printed as
// a blank line.
func classTwoReport(message string, alsoFailing []string, appended string) analysisErrorReport {
	lines := append([]string{message}, alsoFailing...)
	if appended != "" {
		lines = append(lines, appended)
	}
	return analysisErrorReport{lines: lines, exitCode: 2}
}
