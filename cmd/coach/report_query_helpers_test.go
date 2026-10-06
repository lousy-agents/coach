package main

import (
	"encoding/json"
	"slices"
	"strings"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/pkg/codesignal"
)

func hasDiagnostic(report *codesignal.Report, kind, path string) bool {
	for _, d := range report.Diagnostics {
		if d.Kind == kind && d.Path == path {
			return true
		}
	}
	return false
}

// sourceScopeForPath reads the customer-facing source_scope emitted with a
// signal. It intentionally decodes the public JSON document rather than a Go
// report type so this acceptance suite requires the label to be serialized.
func sourceScopeForPath(stdout []byte, path string) string {
	var document struct {
		Signals []struct {
			Path        string `json:"path"`
			SourceScope string `json:"source_scope"`
		} `json:"signals"`
	}
	Expect(json.Unmarshal(stdout, &document)).To(Succeed(), "stdout should be a JSON CodeSignal report: %s", stdout)

	for _, signal := range document.Signals {
		if signal.Path == path {
			return signal.SourceScope
		}
	}
	return ""
}

func signalsForPath(report *codesignal.Report, path string) []codesignal.Signal {
	var matches []codesignal.Signal
	for _, sig := range report.Signals {
		if sig.Path == path {
			matches = append(matches, sig)
		}
	}
	return matches
}

// noFindingsHeadline returns the rendered "No active CodeSignal findings"
// verdict line, or the report's first line when no such verdict was printed.
func noFindingsHeadline(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "No active CodeSignal findings") {
			return line
		}
	}
	return strings.SplitN(text, "\n", 2)[0]
}

func decodeCoachReport(stdout []byte) *codesignal.Report {
	var report codesignal.Report
	ExpectWithOffset(1, json.Unmarshal(stdout, &report)).To(Succeed(), "stdout should be one JSON report: %s", stdout)
	return &report
}

// firstSignalIndex returns the index of the first signal whose RuleID is one
// of ruleIDs, or -1 when none matches.
func firstSignalIndex(signals []codesignal.Signal, ruleIDs ...string) int {
	for i, sig := range signals {
		if slices.Contains(ruleIDs, sig.RuleID) {
			return i
		}
	}
	return -1
}

func reportDiagnosticKinds(report *codesignal.Report) []string {
	kinds := make([]string, len(report.Diagnostics))
	for i, d := range report.Diagnostics {
		kinds[i] = d.Kind
	}
	return kinds
}
