package render

import "github.com/lousy-agents/coach/pkg/codesignal"

// worktreeScanClauses carries each run-level cause a --worktree scan recorded
// for paths it cannot see, so the verdict names it. A reason that hides no
// in-scope source is disclosed elsewhere and does not qualify the verdict. A
// reason appears once however many roots it names; an entry with no reason, from
// a report written before the field existed, is always named.
func worktreeScanClauses(diagnostics []codesignal.Diagnostic) []string {
	var clauses []string
	namedReasons := map[string]bool{}
	for _, d := range diagnostics {
		if d.Kind != codesignal.DiagKindWorktreeScanIncomplete || !d.QualifiesVerdict() {
			continue
		}
		repeated := d.Reason != "" && namedReasons[d.Reason]
		namedReasons[d.Reason] = true
		if !repeated {
			clauses = append(clauses, d.Message)
		}
	}
	return clauses
}
