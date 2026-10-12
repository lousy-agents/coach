package codesignal

import (
	"strings"

	"github.com/lousy-agents/coach/pkg/domain"
)

// projectLifecycleIndeterminacy breaks the indeterminacy condition out per
// cause so each degraded change can be attributed to the responsible side(s).
// baseIncomplete and inconsistentBase are mutually exclusive.
type projectLifecycleIndeterminacy struct {
	headIncomplete   bool
	baseIncomplete   bool
	inconsistentBase bool
	headRevision     string
	baseRevision     string
}

func (s projectLifecycleIndeterminacy) any() bool {
	return s.headIncomplete || s.baseIncomplete || s.inconsistentBase
}

// degradedProjectChangeDiagnostics emits one Diagnostic per side the state
// implicates, so a consumer keying off Side never misses a side when both
// are independently incomplete.
func degradedProjectChangeDiagnostics(path string, state projectLifecycleIndeterminacy) []Diagnostic {
	var diagnostics []Diagnostic
	if state.headIncomplete {
		diagnostics = append(diagnostics, Diagnostic{
			Path:     path,
			Kind:     DiagKindProjectChangeLifecycleIndeterminate,
			Message:  "project change lifecycle is \"unknown\": " + sideRevisionLabel("head", state.headRevision) + " project analysis coverage is incomplete",
			Side:     "head",
			Revision: state.headRevision,
		})
	}
	if state.baseIncomplete {
		diagnostics = append(diagnostics, Diagnostic{
			Path:     path,
			Kind:     DiagKindProjectChangeLifecycleIndeterminate,
			Message:  "project change lifecycle is \"unknown\": " + sideRevisionLabel("base", state.baseRevision) + " project analysis coverage is incomplete",
			Side:     "base",
			Revision: state.baseRevision,
		})
	}
	if state.inconsistentBase {
		diagnostics = append(diagnostics, Diagnostic{
			Path:     path,
			Kind:     DiagKindProjectChangeLifecycleIndeterminate,
			Message:  "project change lifecycle is \"unknown\": " + sideRevisionLabel("base", state.baseRevision) + " supplied project observations without a completed base analysis",
			Side:     "base",
			Revision: state.baseRevision,
		})
	}
	return diagnostics
}

func projectLifecycleState(input Input) (state projectLifecycleIndeterminacy, diagnostics []Diagnostic) {
	state.headRevision = input.Scope.Revision
	state.baseRevision = input.Scope.Base

	// Complete coverage is required before any normal lifecycle claim.
	if !completeProjectCoverage(input.ProjectCoverage) {
		state.headIncomplete = true
	}
	if input.ProjectBaseAnalyzed && !completeProjectCoverage(input.BaseProjectCoverage) {
		state.baseIncomplete = true
	}
	// Non-empty base observations without ProjectBaseAnalyzed are inconsistent.
	// A non-nil empty slice is not: callers commonly initialize with make/append.
	if !input.ProjectBaseAnalyzed && len(input.BaseProjectChanges) > 0 {
		state.inconsistentBase = true
	}
	if input.ProjectCoverage != nil && !input.ProjectCoverage.Complete {
		diagnostics = append(diagnostics, Diagnostic{
			Kind:    DiagKindProjectCoverageIncomplete,
			Message: "project analysis coverage is incomplete; project observations may be partial",
		})
	}
	if state.any() {
		diagnostics = append(diagnostics, Diagnostic{
			Kind:    DiagKindProjectLifecycleIndeterminate,
			Message: projectLifecycleDiagnosticMessage(input),
		})
	}
	return state, diagnostics
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
