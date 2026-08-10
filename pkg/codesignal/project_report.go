package codesignal

import "github.com/lousy-agents/coach/pkg/semantics"

// ProjectChange is one project-level observation derived from
// pkg/projectmodel facts -- the cross-file/cross-module analog of Signal.
// Unlike Signal (scoped to one FileChange, deduplicated by a composite
// rule/path/subject/evidence key with per-occurrence ordinals), a
// ProjectChange's SemanticKey is itself the stable lifecycle identity: at
// most one ProjectChange per SemanticKey exists on each side (head/base) of
// a comparison. ProjectChange never wraps a synthetic FileChange -- it
// carries its own PrimaryAnchor/RelatedLocations/PathSteps instead.
type ProjectChange struct {
	SemanticKey      string     `json:"semantic_key"`
	ID               string     `json:"id"`
	Fingerprint      string     `json:"fingerprint"`
	RuleID           string     `json:"rule_id"`
	RuleVersion      string     `json:"rule_version"`
	BackendVersion   string     `json:"backend_version,omitempty"`
	AlgorithmVersion string     `json:"algorithm_version,omitempty"`
	ConfigDigest     string     `json:"config_digest,omitempty"`
	Kind             string     `json:"kind"`
	Category         Category   `json:"category"`
	Severity         Severity   `json:"severity"`
	Confidence       Confidence `json:"confidence"`
	Lifecycle        Lifecycle  `json:"lifecycle"`

	// Changed is derived by the lifecycle classifier from causal evidence and
	// the primary anchor. A producer may set it while constructing an
	// observation, but Build recomputes it whenever a comparable base exists.
	Changed              bool              `json:"changed"`
	CausalEvidenceDigest string            `json:"causal_evidence_digest,omitempty"`
	PrimaryAnchor        ProjectLocation   `json:"primary_anchor"`
	RelatedLocations     []ProjectLocation `json:"related_locations,omitempty"`
	PathSteps            []ProjectPathStep `json:"path_steps,omitempty"`
	CoverageRefs         []string          `json:"coverage_refs,omitempty"`
	Evidence             string            `json:"evidence,omitempty"`
	WhyItMatters         string            `json:"why_it_matters,omitempty"`
	Recommendation       string            `json:"recommendation,omitempty"`
	SuggestedSkill       string            `json:"suggested_skill,omitempty"`
	Provenance           Provenance        `json:"provenance"`
}

// ProjectLocation anchors a ProjectChange (or one step/related point) to a
// specific file and position -- Signal's Location is file-implicit (the
// enclosing Signal.Path); ProjectChange has no single enclosing file, so
// every location must carry its own Path.
type ProjectLocation struct {
	Path     string             `json:"path"`
	Location semantics.Location `json:"location"`
}

// ProjectPathStep is one node in a project observation's causal or
// reachability path. SourceLocations retain the repository evidence for that
// node without flattening the path into an ambiguous list of files.
type ProjectPathStep struct {
	NodeID          string            `json:"node_id"`
	DisplayName     string            `json:"display_name,omitempty"`
	Resolution      string            `json:"resolution,omitempty"`
	Confidence      Confidence        `json:"confidence,omitempty"`
	SourceLocations []ProjectLocation `json:"source_locations,omitempty"`
}

// ProjectFact is a facts-only project observation (e.g. possible call
// reachability). Unlike ProjectChange it is never lifecycle-classified, never
// appears in ProjectSummary active counters, and must not be encoded as a
// synthetic ProjectChange with a placeholder anchor.
type ProjectFact struct {
	Kind         string            `json:"kind"`
	SemanticKey  string            `json:"semantic_key,omitempty"`
	PathSteps    []ProjectPathStep `json:"path_steps,omitempty"`
	CoverageRefs []string          `json:"coverage_refs,omitempty"`
	Evidence     string            `json:"evidence,omitempty"`
	Provenance   Provenance        `json:"provenance"`
}

// ProjectSummary counts ProjectChanges across a Report the same way Summary
// counts Signals. It is a separate, always-optional struct (never merged
// into Summary) so a disabled/default Report's existing Summary fields
// never change shape or gain silently-always-zero project fields.
//
// Report.ProjectSummary is non-nil whenever Options.ProjectEnabled is true,
// even when there are zero ProjectChanges on either side of the comparison
// -- enabling the mode, not having something to report, is what makes the
// field appear, so a caller can distinguish "project analysis ran and found
// nothing" (present, all-zero) from "project analysis did not run"
// (absent, omitempty). Fact-only observations never increment these counters.
type ProjectSummary struct {
	ActiveChanges     int `json:"active_changes"`
	IntroducedChanges int `json:"introduced_changes"`
	ExistingChanges   int `json:"existing_changes"`
	ResolvedChanges   int `json:"resolved_changes"`
	BaselineChanges   int `json:"baseline_changes"`
}
