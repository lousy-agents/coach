package main

import (
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/terminal"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"

	"os"

	"strings"
)

func runCodesignalScan(dir string, f codesignalFlags, stdout, stderr *os.File, budget scanOfferBudget) int {
	report, err := runOneScan(dir, f, stderr)
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
	return renderScanResult(report, f.failOnIncompleteCoverage, f.format, stdout, stderr)
}

// setupResidueDisclosure renders AC-SET-7's "identify files that may have
// changed" for a failed setup. ResidueUnknown is not a quieter version of an
// empty ChangedPaths: it means Coach could not read what changed at all, and
// the fallback path it carries is the working directory itself -- which
// renders repository-relative as a bare ".", indistinguishable from a precise
// finding. Saying so plainly is the difference between "nothing changed" and
// "Coach does not know", which is exactly the distinction SetupOutcome's own
// contract asks callers to preserve.
func setupResidueDisclosure(result tssetup.CompilerSetupOfferResult) string {
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
func wrapScanAnalysisError(err error, dir, revision, configPath string, stderr *os.File) error {
	var unresolved *tstoolchain.CompilerUnresolvedError
	if errors.As(err, &unresolved) {
		return tssetup.WrapCompilerUnresolvedErrorWithReadiness(unresolved, dir, revision, configPath)
	}
	var runtimeErr *tstoolchain.RuntimeUnresolvedError
	if errors.As(err, &runtimeErr) {
		return runtimeErr
	}
	fmt.Fprintf(stderr, "coach codesignal: analysis failed: %s\n", err)
	return nil
}

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
