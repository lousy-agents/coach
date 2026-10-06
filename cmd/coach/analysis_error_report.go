package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/terminal"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

// classifyAnalysisError never sees a project_backend_unavailable error --
// prepareProjectAnalysis handles that case separately by returning
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
	report := analysisErrorReportFor(err, language, terminal.HasControllingTerminal(os.Stdin) && !noInteractive)
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
	var unresolvedWithReadiness *tssetup.CompilerUnresolvedErrorWithReadiness
	if errors.As(err, &unresolvedWithReadiness) {
		remediation := tssetup.PrepareCompilerRemediationWithReadiness(unresolvedWithReadiness.Code, unresolvedWithReadiness.ConfigPath, unresolvedWithReadiness.Readiness)
		if remediation == "" {
			remediation = tssetup.ScanSetupOfferRemediation(unresolvedWithReadiness.Code, unresolvedWithReadiness.ConfigPath, unresolvedWithReadiness.Readiness)
		}
		return classTwoReport(unresolvedWithReadiness.RemediationLine(), nil,
			tssetup.AppendedRemediationLine(hasControllingTerminal, language, remediation))
	}
	var unresolved *tstoolchain.CompilerUnresolvedError
	if errors.As(err, &unresolved) {
		return classTwoReport(unresolved.RemediationLine(), nil,
			tssetup.AppendedRemediationLine(hasControllingTerminal, language, tssetup.PrepareCompilerRemediation(unresolved.Code, unresolved.ConfigPath)))
	}
	var runtimeErr *tstoolchain.RuntimeUnresolvedError
	if errors.As(err, &runtimeErr) {
		return classTwoReport(runtimeErr.RemediationLine(), nil, "")
	}
	var configErrWithReadiness *tssetup.ProjectConfigErrorWithReadiness
	if errors.As(err, &configErrWithReadiness) {
		return classTwoReport(configErrWithReadiness.Message,
			tssetup.AlsoFailingGapLines(configErrWithReadiness.Readiness, configErrWithReadiness.ConfigPath),
			tssetup.AppendedRemediationLine(hasControllingTerminal, language, tssetup.SuggestProjectConfigRemediation(language)))
	}
	var configErr *projectconfig.ConfigError
	if errors.As(err, &configErr) {
		return classTwoReport(configErr.Message, nil,
			tssetup.AppendedRemediationLine(hasControllingTerminal, language, tssetup.SuggestProjectConfigRemediation(language)))
	}
	var opErr *gitrepo.OperationalError
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
