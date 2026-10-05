package codesignal

import (
	"strings"

	"github.com/lousy-agents/coach/pkg/domain"
)

func projectLifecycleState(input Input) (indeterminate bool, diagnostics []Diagnostic) {
	// Complete coverage is required before any normal lifecycle claim.
	if !completeProjectCoverage(input.ProjectCoverage) {
		indeterminate = true
	}
	if input.ProjectBaseAnalyzed && !completeProjectCoverage(input.BaseProjectCoverage) {
		indeterminate = true
	}
	// Non-empty base observations without ProjectBaseAnalyzed are inconsistent.
	// A non-nil empty slice is not: callers commonly initialize with make/append.
	if !input.ProjectBaseAnalyzed && len(input.BaseProjectChanges) > 0 {
		indeterminate = true
	}
	if input.ProjectCoverage != nil && !input.ProjectCoverage.Complete {
		diagnostics = append(diagnostics, Diagnostic{
			Kind:    DiagKindProjectCoverageIncomplete,
			Message: "project analysis coverage is incomplete; project observations may be partial",
		})
	}
	if indeterminate {
		diagnostics = append(diagnostics, Diagnostic{
			Kind:    DiagKindProjectLifecycleIndeterminate,
			Message: projectLifecycleDiagnosticMessage(input),
		})
	}
	return indeterminate, diagnostics
}

func completeProjectCoverage(coverage *domain.Coverage) bool {
	return coverage != nil && coverage.Complete
}

func projectLifecycleDiagnosticMessage(input Input) string {
	reasons := make([]string, 0, 2)
	if input.ProjectCoverage == nil {
		reasons = append(reasons, "head coverage unavailable")
	} else if !input.ProjectCoverage.Complete {
		reasons = append(reasons, "head coverage incomplete")
	}
	// Only blame the base side when a base model was analyzed or base
	// observations were actually supplied. Baseline runs and head-only
	// diffs never expect base coverage.
	baseSideExpected := input.ProjectBaseAnalyzed || len(input.BaseProjectChanges) > 0
	if baseSideExpected {
		if !input.ProjectBaseAnalyzed || input.BaseProjectCoverage == nil {
			reasons = append(reasons, "base coverage unavailable")
		} else if !input.BaseProjectCoverage.Complete {
			reasons = append(reasons, "base coverage incomplete")
		}
	}
	if len(reasons) == 0 {
		return "project lifecycle is indeterminate"
	}
	return "project lifecycle is indeterminate: " + strings.Join(reasons, "; ")
}
