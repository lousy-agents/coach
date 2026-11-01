package codesignal

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics"
)

// Diagnostic reports a problem observed while building a Report.
type Diagnostic struct {
	Path     string              `json:"path"`
	Kind     string              `json:"kind"`
	Message  string              `json:"message"`
	Location *semantics.Location `json:"location,omitempty"`

	// Side identifies which comparison side this diagnostic concerns: "head"
	// or "base".
	Side string `json:"side,omitempty"`
	// Revision is Side's git revision (e.g. commit SHA), so the diagnostic's
	// text-named side/revision has a machine-readable counterpart.
	Revision string `json:"revision,omitempty"`
}

// Diagnostic.Kind values projectLifecycleState (project_lifecycle_state.go) emits when
// project analysis did not complete, on either side of a diff.
const (
	DiagKindProjectCoverageIncomplete     = "project_coverage_incomplete"
	DiagKindProjectLifecycleIndeterminate = "project_lifecycle_indeterminate"
	DiagKindWorktreeChangesNotAnalyzed    = "worktree_changes_not_analyzed"
)

// Diagnostic.Kind values EvaluateTypeScriptLayerViolations emits for import
// edges whose target layer rules cannot place. An unresolved target could be
// a broken alias that hides a forbidden edge, so it makes the layer verdict
// incomplete. The aggregated not-evaluated count covers external packages,
// non-source files, and aliases the sidecar does not follow; it is
// informational by decision, and docs/cli-codesignal.md states the gap.
const (
	DiagKindProjectImportUnresolved            = "project_import_unresolved"
	DiagKindProjectExternalImportsNotEvaluated = "project_external_imports_not_evaluated"
)

// Dirty-tree notices other than "changes were not analyzed".
// These kind strings must not contain "not_analyzed".
const (
	DiagKindWorktreeNotClean                    = "worktree_not_clean"
	DiagKindWorktreeStatusCheckFailed           = "worktree_status_check_failed"
	DiagKindWorktreeReportReflectsCommittedHEAD = "worktree_report_reflects_committed_head"
)

// SortDiagnostics orders diagnostics by path, kind, location, then message.
// Callers that append diagnostics after Build must use this so report order
// stays identical to Build's own sortDiagnostics pass.
func SortDiagnostics(diagnostics []Diagnostic) {
	sortDiagnostics(diagnostics)
}

func sortDiagnostics(diagnostics []Diagnostic) {
	sort.SliceStable(diagnostics, func(i, j int) bool {
		a, b := diagnostics[i], diagnostics[j]

		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		if cmp := compareLocationPtr(a.Location, b.Location); cmp != 0 {
			return cmp < 0
		}
		return a.Message < b.Message
	})
}
