package codesignal

import "github.com/lousy-agents/coach/pkg/projectmodel"

// ProjectScopeReport is the report-level wrapper for project_scope. It
// adds head/base revision identity around the per-revision scope data that
// projectmodel.ProjectScope provides. inclusion_rule and pattern_set are
// top-level (same for both revisions); head and base each carry their own
// revision SHA plus the per-revision roots/layer classification.
//
// Populated only for schema-2 TypeScript reports; omitted for Go reports
// via omitempty on Report.ProjectScope.
type ProjectScopeReport struct {
	InclusionRule string                      `json:"inclusion_rule"`
	PatternSet    string                      `json:"pattern_set"`
	Head          ProjectScopeRevisionReport  `json:"head"`
	Base          *ProjectScopeRevisionReport `json:"base,omitempty"`
}

// ProjectScopeRevisionReport is the per-revision part of project_scope,
// combining the revision SHA with the scope classification results.
// Roots reuses projectmodel.ProjectScopeRoot so the JSON tags stay
// consistent with the projectmodel package (root, candidate_files,
// analyzed_files).
type ProjectScopeRevisionReport struct {
	Revision        string                          `json:"revision"`
	Roots           []projectmodel.ProjectScopeRoot `json:"roots"`
	MatchedLayers   []string                        `json:"matched_layers"`
	UnmatchedLayers []string                        `json:"unmatched_layers"`
}

// ProjectNextAction is one recommended next step surfaced in
// project_next_actions. Kind values: record_baseline,
// review_policy_coverage, inspect_diagnostics.
type ProjectNextAction struct {
	Kind string `json:"kind"`
}
