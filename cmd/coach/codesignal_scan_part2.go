package main

import (
	"errors"
	"fmt"

	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

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
func renderScanResult(report *codesignal.Report, f codesignalFlags, stdout, stderr *os.File) int {
	if report == nil {
		return 1
	}
	incompleteCoverageFails := f.failOnIncompleteCoverage && codesignal.RequiredCoverageIncomplete(report)
	view, err := report.Narrow(narrowOptionsFor(f))
	if err != nil {
		fmt.Fprintf(stderr, "coach codesignal: narrowing report: %s\n", err)
		return 1
	}
	renderOptions := codesignalcli.RenderOptions{SeeAllCommand: seeAllCommand(f.args)}
	if exitCode := renderReport(view, f.format, renderOptions, stdout, stderr); exitCode != 0 {
		return exitCode
	}
	if incompleteCoverageFails {
		return 3
	}
	return 0
}

func narrowOptionsFor(f codesignalFlags) codesignal.NarrowOptions {
	var opts codesignal.NarrowOptions
	if f.minSeveritySet {
		opts.MinSeverity = codesignal.Severity(f.minSeverity)
	}
	if f.topSet {
		opts.Top, _ = parseTopCap(f.top)
	}
	return opts
}
