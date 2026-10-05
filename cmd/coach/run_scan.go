package main

import (
	"os"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

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

func runOneScan(dir string, f codesignalFlags, stderr *os.File) (*codesignal.Report, error) {
	if f.baseline {
		return runBaselineAnalysis(dir, f, stderr)
	}
	return runDiffAnalysis(dir, f, stderr)
}
