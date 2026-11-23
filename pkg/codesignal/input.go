package codesignal

import (
	"github.com/lousy-agents/coach/pkg/domain"
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
	ProjectChanges      []ProjectChange  `json:"project_changes,omitempty"`
	BaseProjectChanges  []ProjectChange  `json:"base_project_changes,omitempty"`
	ProjectFacts        []ProjectFact    `json:"project_facts,omitempty"`
	ProjectCoverage     *domain.Coverage `json:"project_coverage,omitempty"`
	BaseProjectCoverage *domain.Coverage `json:"base_project_coverage,omitempty"`

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

	// UndeterminedContinuity lists rename/copy pairs whose old-path
	// continuity was not determined. A project change that would otherwise
	// be introduced at Path, or resolved at PreviousPath, is classified
	// "unknown" instead: a path-keyed identity cannot tell a move from a
	// fix plus a new defect.
	UndeterminedContinuity []PathContinuity `json:"undetermined_continuity,omitempty"`

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
	HeadProjectScope *domain.ProjectScope `json:"head_project_scope,omitempty"`
	BaseProjectScope *domain.ProjectScope `json:"base_project_scope,omitempty"`

	// HeadLayerMembership and BaseLayerMembership carry each revision's count
	// of analyzed files outside every declared layer. Nil means it was not
	// computed for that revision, which is not the same as zero.
	HeadLayerMembership *domain.LayerMembership `json:"head_layer_membership,omitempty"`
	BaseLayerMembership *domain.LayerMembership `json:"base_layer_membership,omitempty"`
	// LayerForbiddenImportsDeclared is whether the project policy declares
	// any forbidden_imports, the only layer rule that cannot see files
	// outside every layer.
	LayerForbiddenImportsDeclared bool `json:"layer_forbidden_imports_declared,omitempty"`

	// Phase-specific coverage observations for each analyzed revision. A nil
	// pointer means the phase was not observed (treat as not_run in D7 mapping).
	HeadModelCoverage        *domain.Coverage `json:"head_model_coverage,omitempty"`
	BaseModelCoverage        *domain.Coverage `json:"base_model_coverage,omitempty"`
	HeadBypassCoverage       *domain.Coverage `json:"head_bypass_coverage,omitempty"`
	BaseBypassCoverage       *domain.Coverage `json:"base_bypass_coverage,omitempty"`
	HeadReachabilityCoverage *domain.Coverage `json:"head_reachability_coverage,omitempty"`
	BaseReachabilityCoverage *domain.Coverage `json:"base_reachability_coverage,omitempty"`
}

// Scope identifies the repository and revision range an Input covers.
type Scope struct {
	Repository   string `json:"repository,omitempty"`
	Revision     string `json:"revision,omitempty"`
	Base         string `json:"base,omitempty"`
	AppliedScope string `json:"applied_scope,omitempty"`
	Baseline     bool   `json:"baseline,omitempty"`
	// Worktree is present only for a --worktree scan.
	Worktree *WorktreeScope `json:"worktree,omitempty"`
	// Policy is always serialized; null means no policy was used for the analysis. A policy that was
	// found but not analyzed is named by a project_backend_unavailable diagnostic instead.
	Policy *PolicyScope `json:"policy"`
}

// PolicyScope names the policy file a scan used.
type PolicyScope struct {
	// Path is repository-relative.
	Path string `json:"path"`
	// Digest is the pcfg_-prefixed sha256 of the policy bytes the scan used.
	Digest string `json:"digest"`
	// MatchesHead is true when the policy selected from disk has the same path
	// and digest as the policy selected at HEAD.
	MatchesHead bool `json:"matches_head"`
	// Cause says why the scan used this policy; its values are part of the JSON contract. A producer always
	// sets it, and a report from an older version that lacks it decodes to PolicyCauseUnset.
	Cause PolicyCause `json:"cause"`
}

// PolicyCause says how the policy a scan used relates to the one committed at HEAD. It is a closed enum
// serialized as scope.policy.cause.
type PolicyCause string

const (
	// PolicyCauseUnset is the zero value of a report decoded without a cause: the text reads MatchesHead alone.
	PolicyCauseUnset PolicyCause = ""
	// PolicyCauseSameAsHead means the policy on disk is the committed one.
	PolicyCauseSameAsHead PolicyCause = "same_as_head"
	// PolicyCauseDiffersFromHead means the scan used a policy on disk that differs from the committed one.
	PolicyCauseDiffersFromHead PolicyCause = "differs_from_head"
	// PolicyCauseNotCommitted means the scan used a policy on disk with no committed copy at HEAD.
	PolicyCauseNotCommitted PolicyCause = "not_committed"
	// PolicyCauseMissingOnDisk means the policy is committed at HEAD and absent on disk, so the scan used the committed copy.
	PolicyCauseMissingOnDisk PolicyCause = "missing_on_disk"
)

// WorktreeScope says what a --worktree scan read from disk and what the working
// tree changes.
type WorktreeScope struct {
	// Revision is the commit the overlay sits on; it equals Scope.Revision.
	Revision string `json:"revision"`
	// Modified, Added, Deleted and Untracked count distinct paths from git
	// status, in every language, ignored paths excluded. A path has one category,
	// checked in that order of precedence: untracked, deleted, added, modified.
	// A rename counts as one deleted path and one added path.
	Modified  int `json:"modified"`
	Added     int `json:"added"`
	Deleted   int `json:"deleted"`
	Untracked int `json:"untracked"`
	// ConfigFiles lists the project configuration files (tsconfig*.json,
	// package.json, go.mod, go.work) the scan read from the working tree
	// instead of the committed revision, as sorted repository-relative paths.
	ConfigFiles []string `json:"config_files"`
}

type ChangeStatus string

// PathContinuity is one git rename or copy: PreviousPath at the base
// revision, Path at head.
type PathContinuity struct {
	Path         string `json:"path"`
	PreviousPath string `json:"previous_path"`
}

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
