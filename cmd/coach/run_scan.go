package main

import (
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
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

func runCodesignalScan(dir string, f codesignalFlags, stdin, stdout, stderr *os.File, budget scanOfferBudget, ports scanPorts) int {
	report, err := runOneScan(dir, f, stdin, stderr, ports)
	if err != nil {
		noInteractive := nonInteractiveRequested(f)
		if offerAuthoring(err, f.projectLanguage, authoringInteractive(noInteractive)) == authoringSession {
			return runScanProjectConfigAuthoring(dir, f, err, stdout, stderr, ports)
		}
		if wrapped, ok := scanShouldOfferCompilerSetup(err, noInteractive); budget.allows(scanOfferCompilerSetup) && ok {
			return runScanCompilerSetupOffer(dir, f, stdin, stdout, stderr, wrapped, budget, ports)
		}
		err = wordBlobReadFailure(gitrepo.RunBytes, dir, err, scanPrefix)
		return classifyAnalysisError(err, f.projectLanguage, f.namedPolicy, noInteractive, stderr, dir)
	}
	if result := ports.optionalPreparation(dir, f, stdout, stderr); !shouldRenderAfterOptionalPreparation(result) {
		return 2
	}
	exitCode := renderScanResult(report, f, stdout, stderr)
	if exitCode == 0 {
		offerAuthoringForLayerlessDiscoveredPolicy(dir, f, report, stderr, ports)
	}
	return exitCode
}

func runOneScan(dir string, f codesignalFlags, stdin, stderr *os.File, ports scanPorts) (*codesignal.Report, error) {
	if f.baseline {
		return runBaselineAnalysis(dir, f, stdin, stderr, ports)
	}
	return runDiffAnalysis(dir, f, stdin, stderr, ports)
}
