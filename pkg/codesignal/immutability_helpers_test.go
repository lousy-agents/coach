package codesignal

import (
	"github.com/lousy-agents/coach/pkg/semantics"
)

func snapshotResult(r *semantics.Result) *semantics.Result {
	if r == nil {
		return nil
	}
	cp := *r
	cp.SyntaxErrors = append([]semantics.SyntaxIssue(nil), r.SyntaxErrors...)
	cp.Imports = append([]semantics.ImportFeature(nil), r.Imports...)
	cp.Findings = append([]semantics.Finding(nil), r.Findings...)
	return &cp
}
