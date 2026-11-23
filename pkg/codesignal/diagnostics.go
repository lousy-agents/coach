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

	// Reason, Root, HidesInScope and RootsNotListed describe one worktree_scan_incomplete
	// entry: why git hides part of the working tree, the directory it hides, whether
	// in-scope source is among what it hides, and how many further roots the report
	// does not list. Path stays empty on these entries, because a path counts as an
	// unanalyzed file.
	Reason         string `json:"reason,omitempty"`
	Root           string `json:"root,omitempty"`
	HidesInScope   string `json:"hides_in_scope,omitempty"`
	RootsNotListed int    `json:"roots_not_listed,omitempty"`
}

// Diagnostic.HidesInScope values.
const (
	HidesInScopeYes     = "yes"
	HidesInScopeNo      = "no"
	HidesInScopeUnknown = "unknown"
)

// QualifiesVerdict reports whether a worktree_scan_incomplete diagnostic makes a
// verdict incomplete. Only an explicit "no" does not: a missing value reads as
// "unknown", so a report written before the field existed keeps qualifying. Other
// kinds never carry HidesInScope and are decided by their own Kind.
func (d Diagnostic) QualifiesVerdict() bool {
	return d.HidesInScope != HidesInScopeNo
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
//
// A dynamic import() or require() whose specifier is not a literal names no
// target at all, so it cannot be placed either and makes the verdict incomplete
// the same way.
const (
	DiagKindProjectImportUnresolved            = "project_import_unresolved"
	DiagKindProjectImportNonLiteral            = "project_import_non_literal"
	DiagKindProjectExternalImportsNotEvaluated = "project_external_imports_not_evaluated"
)

// Dirty-tree notices other than "changes were not analyzed".
// These kind strings must not contain "not_analyzed".
const (
	DiagKindWorktreeNotClean                    = "worktree_not_clean"
	DiagKindWorktreeStatusCheckFailed           = "worktree_status_check_failed"
	DiagKindWorktreeReportReflectsCommittedHEAD = "worktree_report_reflects_committed_head"
	DiagKindWorktreeSymlinkSkipped              = "worktree_symlink_skipped"
	DiagKindWorktreeScanIncomplete              = "worktree_scan_incomplete"
)

// DiagKindProjectPolicyAbsentOnDisk names the policy HEAD selects when the working tree holds no file there, so the scan
// did not read it. Path is the policy HEAD selects; scope.policy names the policy the scan used instead, or is null.
const DiagKindProjectPolicyAbsentOnDisk = "project_policy_absent_on_disk"

// DiagnosticOfKind is the first diagnostic of kind, and whether there is one.
func DiagnosticOfKind(diagnostics []Diagnostic, kind string) (Diagnostic, bool) {
	for _, diagnostic := range diagnostics {
		if diagnostic.Kind == kind {
			return diagnostic, true
		}
	}
	return Diagnostic{}, false
}

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
