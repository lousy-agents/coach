package main

import (
	"errors"

	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/tssetup"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

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
func renderScanResult(report *codesignal.Report, failOnIncompleteCoverage bool, format string, stdout, stderr *os.File) int {
	if report == nil {
		return 1
	}
	if exitCode := renderReport(report, format, stdout, stderr); exitCode != 0 {
		return exitCode
	}
	if failOnIncompleteCoverage && codesignal.RequiredCoverageIncomplete(report) {
		return 3
	}
	return 0
}
