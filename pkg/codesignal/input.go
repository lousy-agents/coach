package codesignal

import (
	"github.com/lousy-agents/coach/pkg/projectmodel"
	"github.com/lousy-agents/coach/pkg/semantics"
)

// Input is the unit of work for a Builder.
type Input struct {
	Scope       Scope        `json:"scope"`
	Files       []FileChange `json:"files,omitempty"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
	Coverage    *Coverage    `json:"coverage,omitempty"`

	// ProjectChanges are head-side raw project observations, not yet
	// lifecycle-classified. BaseProjectChanges is the base-side equivalent
	// for a diff-flow comparison (empty/nil for a Repository Baseline
	// run). SemanticKey must be set by the caller on every entry -- it is
	// ProjectChange's lifecycle identity, unlike Signal which derives its
	// key from rule/path/subject/evidence.
	ProjectChanges      []ProjectChange        `json:"project_changes,omitempty"`
	BaseProjectChanges  []ProjectChange        `json:"base_project_changes,omitempty"`
	ProjectFacts        []ProjectFact          `json:"project_facts,omitempty"`
	ProjectCoverage     *projectmodel.Coverage `json:"project_coverage,omitempty"`
	BaseProjectCoverage *projectmodel.Coverage `json:"base_project_coverage,omitempty"`

	// ProjectBaseAnalyzed reports whether a base-side project model was
	// built at all; distinct from len(BaseProjectChanges) > 0, which cannot
	// distinguish a clean base (analyzed, zero changes) from no base having
	// been analyzed. Build uses this flag (not slice length) to decide
	// whether head-only project changes are "introduced" or "unknown" --
	// mirroring how Signal lifecycle classification keys off FileChange.Base's
	// presence rather than whether it produced findings. A non-nil empty
	// BaseProjectChanges with ProjectBaseAnalyzed false is treated as "no
	// base side" (valid for baseline); non-empty changes without the flag
	// are inconsistent and force lifecycle-indeterminate.
	ProjectBaseAnalyzed bool `json:"project_base_analyzed,omitempty"`

	RuntimeKind     string `json:"runtime_kind,omitempty"`
	RuntimeVersion  string `json:"runtime_version,omitempty"`
	RuntimeOrigin   string `json:"runtime_origin,omitempty"`
	CompilerVersion string `json:"compiler_version,omitempty"`
	CompilerOrigin  string `json:"compiler_origin,omitempty"`

	// Language is the project language ("typescript" or "go"). Only
	// "typescript" triggers project_provenance population on the report.
	Language string `json:"language,omitempty"`

	// ConfigDigest is the stable hex digest of the validated project config bytes.
	ConfigDigest string `json:"config_digest,omitempty"`

	// SelectedRoots lists the config roots declared in the project config.
	SelectedRoots []string `json:"selected_roots,omitempty"`

	// AnalyzerProtocolVersion identifies the protocol version the analyzer used
	// to produce project observations. For TypeScript reports, buildProjectProvenance
	// defaults this to 1 when the caller leaves it zero.
	AnalyzerProtocolVersion int    `json:"analyzer_protocol_version,omitempty"`
	AnalyzerVersion         string `json:"analyzer_version,omitempty"`
	AnalyzerDigest          string `json:"analyzer_digest,omitempty"`

	// PackageManager* describe the package manager detected from the analyzed
	// snapshot's committed metadata. Version is omitted when unverifiable from PATH.
	PackageManagerKind    string `json:"package_manager_kind,omitempty"`
	PackageManagerVersion string `json:"package_manager_version,omitempty"`
	PackageManagerOrigin  string `json:"package_manager_origin,omitempty"`

	// HeadProjectScope and BaseProjectScope carry the per-revision scope
	// classification result from the project backend. A nil HeadProjectScope
	// means the backend never produced root_scopes data for that revision
	// (distinct from an analyzed but empty scope). See ProjectBackendResult for
	// the backend contract.
	HeadProjectScope *projectmodel.ProjectScope `json:"head_project_scope,omitempty"`
	BaseProjectScope *projectmodel.ProjectScope `json:"base_project_scope,omitempty"`

	// Phase-specific coverage observations for each analyzed revision. A nil
	// pointer means the phase was not observed (treat as not_run in D7 mapping).
	HeadModelCoverage        *projectmodel.Coverage `json:"head_model_coverage,omitempty"`
	BaseModelCoverage        *projectmodel.Coverage `json:"base_model_coverage,omitempty"`
	HeadBypassCoverage       *projectmodel.Coverage `json:"head_bypass_coverage,omitempty"`
	BaseBypassCoverage       *projectmodel.Coverage `json:"base_bypass_coverage,omitempty"`
	HeadReachabilityCoverage *projectmodel.Coverage `json:"head_reachability_coverage,omitempty"`
	BaseReachabilityCoverage *projectmodel.Coverage `json:"base_reachability_coverage,omitempty"`
}

// Scope identifies the repository and revision range an Input covers.
type Scope struct {
	Repository   string `json:"repository,omitempty"`
	Revision     string `json:"revision,omitempty"`
	Base         string `json:"base,omitempty"`
	AppliedScope string `json:"applied_scope,omitempty"`
	Baseline     bool   `json:"baseline,omitempty"`
}

type ChangeStatus string

// FileChange describes one file's before/after analysis results.
type FileChange struct {
	Path          string            `json:"path"`
	Status        ChangeStatus      `json:"status,omitempty"`
	SourceScope   string            `json:"source_scope,omitempty"`
	Base          *semantics.Result `json:"base,omitempty"`
	Head          *semantics.Result `json:"head,omitempty"`
	ChangedRanges []LineRange       `json:"changed_ranges,omitempty"`
}

// LineRange is a 0-based, inclusive row range.
type LineRange struct {
	StartRow uint `json:"start_row"`
	EndRow   uint `json:"end_row"`
}
