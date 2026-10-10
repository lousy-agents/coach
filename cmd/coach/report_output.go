package main

import (
	"fmt"
	"os"

	"github.com/lousy-agents/coach/internal/codesignalcli/render"
	"github.com/lousy-agents/coach/pkg/codesignal"
)

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
	textOptions := render.TextOptions{SeeAllCommand: seeAllCommand(f.args)}
	if exitCode := renderReport(view, f.format, textOptions, stdout, stderr); exitCode != 0 {
		return exitCode
	}
	if incompleteCoverageFails {
		return 3
	}
	return 0
}

func renderReport(report *codesignal.Report, format string, textOptions render.TextOptions, stdout, stderr *os.File) int {
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

	if _, err := fmt.Fprint(stdout, render.ReportTextWithOptions(report, textOptions)); err != nil {
		fmt.Fprintf(stderr, "coach codesignal: writing report: %s\n", err)
		return 1
	}
	return 0
}
