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

func runCodesignalScan(dir string, f codesignalFlags, stdin, stdout, stderr *os.File, budget scanOfferBudget) int {
	report, err := runOneScan(dir, f, stdin, stderr)
	if err != nil {
		noInteractive := nonInteractiveRequested(f)
		if scanShouldAuthorProjectConfig(err, f.projectLanguage, f.projectConfig, noInteractive) ||
			scanShouldAuthorDiscoveredProjectConfig(err, f, noInteractive) {
			return runScanProjectConfigAuthoring(dir, f, err, stdout, stderr)
		}
		if wrapped, ok := scanShouldOfferCompilerSetup(err, noInteractive); budget.allows(scanOfferCompilerSetup) && ok {
			return runScanCompilerSetupOffer(dir, f, stdin, stdout, stderr, wrapped, budget)
		}
		return classifyAnalysisError(err, f.projectLanguage, noInteractive, stderr)
	}
	if result := runOptionalScanPreparation(dir, f, stdout, stderr); !shouldRenderAfterOptionalPreparation(result) {
		return 2
	}
	exitCode := renderScanResult(report, f, stdout, stderr)
	if exitCode == 0 {
		offerAuthoringForLayerlessDiscoveredPolicy(dir, f, report, stderr)
	}
	return exitCode
}

func runOneScan(dir string, f codesignalFlags, stdin, stderr *os.File) (*codesignal.Report, error) {
	if f.baseline {
		return runBaselineAnalysis(dir, f, stdin, stderr)
	}
	return runDiffAnalysis(dir, f, stdin, stderr)
}
