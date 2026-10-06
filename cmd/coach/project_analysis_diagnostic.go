package main

import (
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func withProjectDiagnostic(report *codesignal.Report, diag *codesignal.Diagnostic) *codesignal.Report {
	if report == nil || diag == nil {
		return report
	}
	out := *report
	out.Diagnostics = append(append([]codesignal.Diagnostic(nil), report.Diagnostics...), *diag)
	codesignal.SortDiagnostics(out.Diagnostics)
	return &out
}
