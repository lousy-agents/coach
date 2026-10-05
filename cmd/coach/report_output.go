package main

import (
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/render"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

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

func renderReport(report *codesignal.Report, format string, stdout, stderr *os.File) int {
	if format == "json" {
		encoded, err := render.ReportJSON(report)
		if err != nil {
			fmt.Fprintf(stderr, "coach codesignal: encoding report: %s\n", err)
			return 1
		}
		if _, err := stdout.Write(encoded); err != nil {
			fmt.Fprintf(stderr, "coach codesignal: writing report: %s\n", err)
			return 1
		}
		return 0
	}

	if _, err := fmt.Fprint(stdout, render.ReportText(report)); err != nil {
		fmt.Fprintf(stderr, "coach codesignal: writing report: %s\n", err)
		return 1
	}
	return 0
}
