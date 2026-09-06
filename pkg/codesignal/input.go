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
