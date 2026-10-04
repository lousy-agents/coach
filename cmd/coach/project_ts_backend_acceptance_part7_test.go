package main

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

// diagnosticMessageForKind returns the Message of the first report
// diagnostic matching kind, or "" if none matches.
func diagnosticMessageForKind(diagnostics []codesignal.Diagnostic, kind string) string {
	for _, d := range diagnostics {
		if d.Kind == kind {
			return d.Message
		}
	}
	return ""
}

// AC-12.
func assertReachabilityNeverSignalOrChange(report *codesignal.Report) {
	for _, signal := range report.Signals {
		Expect(signal.RuleID).NotTo(Equal("possible_call_reachability"), "reachability must never surface as a Signal, got %+v", signal)
		Expect(signal.Kind).NotTo(Equal("possible_call_reachability"), "reachability must never surface as a Signal, got %+v", signal)
	}
	for _, change := range report.ProjectChanges {
		Expect(change.RuleID).NotTo(Equal("possible_call_reachability"), "reachability must never surface as a ProjectChange, got %+v", change)
		Expect(change.Kind).NotTo(Equal("possible_call_reachability"), "reachability must never surface as a ProjectChange, got %+v", change)
	}
}

func projectChangeRuleIDs(changes []codesignal.ProjectChange) map[string]bool {
	seen := map[string]bool{}
	for _, change := range changes {
		seen[change.RuleID] = true
	}
	return seen
}
