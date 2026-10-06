package codesignalcli

import (
	"github.com/lousy-agents/coach/pkg/codesignal"
)

func hasDiagnostic(diagnostics []codesignal.Diagnostic, path, kind string) bool {
	for _, d := range diagnostics {
		if d.Path == path && d.Kind == kind {
			return true
		}
	}
	return false
}

func subjectsByPath(signals []codesignal.Signal, ruleID string) map[string]string {
	subjects := map[string]string{}
	for _, sig := range signals {
		if sig.RuleID == ruleID {
			subjects[sig.Path] = sig.Subject
		}
	}
	return subjects
}
