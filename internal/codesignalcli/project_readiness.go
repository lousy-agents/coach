package codesignalcli

import (
	"errors"
)

const (
	ReadinessSchemaVersion   = "1"
	defaultProjectConfigPath = "project.json"
)

// MinimumSupportedNodeMajor is the floor this build of coach requires for
// TypeScript readiness checks, mirroring js/semantics/package.json's
// restated engines minimum (">=24"). TestedNodeMajor is the Node major this
// repository's own CI actually exercises (mise.toml's node pin). Both are
// compiled-in constants, not read from either file at runtime: an analyzed
// repository has neither this repo's package.json nor its mise.toml, and a
// runtime read would conflate "the host running coach" with "the host being
// checked." See TestNodeVersionConstantsMatchDeclaredPins for the pinned
// invariant these must satisfy.
const (
	MinimumSupportedNodeMajor = 24
	TestedNodeMajor           = 24
)

type ReadinessState string

const (
	ReadinessPass       ReadinessState = "pass"
	ReadinessFail       ReadinessState = "fail"
	ReadinessNotChecked ReadinessState = "not_checked"
)

type ReadinessStatus string

const (
	StatusOutsideSupport    ReadinessStatus = "outside_support"
	StatusNeedsPrerequisite ReadinessStatus = "needs_prerequisite"
	StatusNeedsPolicy       ReadinessStatus = "needs_policy"
	StatusReadyWithLimits   ReadinessStatus = "ready_with_limits"
	StatusReady             ReadinessStatus = "ready"
)

// Gap codes map 1:1 to a ReadinessStatus via statusForGapCode. The
// package-manager codes are declared here as vocabulary that later work will
// plug real detection into; checkPackageManager never produces them itself
// yet. The compiler codes are produced by resolveCompiler
// (project_ts_compiler_resolve.go).
const (
	GapUnsupportedRepositoryShape       = "unsupported_repository_shape"
	GapNodeMissing                      = "node_missing"
	GapNodeBelowMinimum                 = "node_below_minimum"
	GapTypescriptCompilerMissing        = "typescript_compiler_missing"
	GapTypescriptVersionMismatch        = "typescript_version_mismatch"
	GapTypescriptVersionConflict        = "typescript_version_conflict"
	GapPackageManagerAmbiguous          = "package_manager_ambiguous"
	GapPackageManagerConfigUnverifiable = "package_manager_config_unverifiable"
	GapPolicyMissing                    = "policy_missing"
	GapPolicyInvalid                    = "policy_invalid"
)

// WarnNodeUntested is not part of the gap-code vocabulary: it never appears
// in gaps[] or drives a status above ready_with_limits. It marks a
// ReadinessCheck.Code on a passing node check whose major differs from
// TestedNodeMajor -- the limit-class condition contributes only
// ready_with_limits, alongside a relevant dirty worktree.
const WarnNodeUntested = "node_untested"

// WarnCompilerDeclarationMismatch is the limit-class warning when a
// selected root's manifest declares a typescript version other than the
// compiler the scan will use. A winning non-project origin warns for any
// differing declaration (range or exact); a winning project origin warns
// only for a stale exact pin -- a range is never warned about, and range
// satisfaction is never evaluated. It never appears in gaps[] and carries
// no next action: Coach never edits manifests.
const WarnCompilerDeclarationMismatch = "compiler_declaration_mismatch"

// ReadinessRootFinding names one selected policy root's compiler-origin
// result. Roots are repository-relative, never absolute host paths.
type ReadinessRootFinding struct {
	Root    string `json:"root"`
	Version string `json:"version,omitempty"`
}

// ReadinessOriginFinding names one compiler-resolution origin and the
// candidate class it yielded. It reaches the customer as remediation text
// only, never as part of the frozen checks.compiler JSON surface.
type ReadinessOriginFinding struct {
	Origin string
	Class  string
}

// ReadinessDeclarationMismatch names one selected root whose manifest
// declares a typescript version other than the compiler the scan will use.
type ReadinessDeclarationMismatch struct {
	Root     string
	Declared string
}

// ReadinessCheck is one entry in ReadinessChecks. Which optional fields
// accompany which code is the frozen compiler-check contract, pinned by
// cmd/coach's aggregation acceptance table; the json:"-" fields never
// serialize and reach the customer as rendered text only.
type ReadinessCheck struct {
	State             ReadinessState           `json:"state"`
	Code              string                   `json:"code,omitempty"`
	Version           string                   `json:"version,omitempty"`
	ExpectedVersion   string                   `json:"expected_version,omitempty"`
	FoundVersion      string                   `json:"found_version,omitempty"`
	SupportedVersions []string                 `json:"supported_versions,omitempty"`
	RootFindings      []ReadinessRootFinding   `json:"root_findings,omitempty"`
	Detail            string                   `json:"detail,omitempty"`
	DeclaredVersion   string                   `json:"-"`
	DeclarationOrigin string                   `json:"-"`
	OriginFindings    []ReadinessOriginFinding `json:"-"`

	DeclarationMismatches []ReadinessDeclarationMismatch `json:"-"`
}

// ReadinessChecks is the fixed set of independently discoverable checks:
// every field always runs and reports pass/fail/not_checked, regardless of
// any other field's outcome.
type ReadinessChecks struct {
	ProjectShape   ReadinessCheck `json:"project_shape"`
	Policy         ReadinessCheck `json:"policy"`
	Node           ReadinessCheck `json:"node"`
	Compiler       ReadinessCheck `json:"compiler"`
	PackageManager ReadinessCheck `json:"package_manager"`
}

type ReadinessGap struct {
	Code string `json:"code"`
}

// ReadinessWarning never blocks readiness on its own, elevating status only
// as far as ready_with_limits. Entries are discriminated by Code, and each
// code populates its own subset of the fields below.
type ReadinessWarning struct {
	Code              string `json:"code"`
	FoundMajor        int    `json:"found_major,omitempty"`
	TestedMajor       int    `json:"tested_major,omitempty"`
	FloorMajor        int    `json:"floor_major,omitempty"`
	DeclaredVersion   string `json:"declared_version,omitempty"`
	FoundVersion      string `json:"found_version,omitempty"`
	DeclarationOrigin string `json:"declaration_origin,omitempty"`
	Root              string `json:"root,omitempty"`
}

type ReadinessNextAction struct {
	Kind string `json:"kind"`
}

// ReadinessDirtyWorktree reports uncommitted/untracked paths relevant to the
// readiness result. Its presence is informational only: it never feeds into
// any check, and RelevantChanges contributes only the ready_with_limits
// limit class, never a gap.
type ReadinessDirtyWorktree struct {
	RelevantChanges bool     `json:"relevant_changes"`
	Paths           []string `json:"paths"`
}

// ReadinessResult is the read-only output of CheckProjectReadiness, rendered
// verbatim (same struct, no drift) by both RenderReadinessText and
// RenderReadinessJSON.
type ReadinessResult struct {
	SchemaVersion string                 `json:"schema_version"`
	Status        ReadinessStatus        `json:"status"`
	Language      string                 `json:"language"`
	Revision      string                 `json:"revision"`
	DirtyWorktree ReadinessDirtyWorktree `json:"dirty_worktree"`
	Checks        ReadinessChecks        `json:"checks"`
	Gaps          []ReadinessGap         `json:"gaps"`
	Warnings      []ReadinessWarning     `json:"warnings"`
	NextActions   []ReadinessNextAction  `json:"next_actions"`
}

// CheckProjectReadiness produces a read-only TypeScript project-readiness
// result at revision. Snapshot checks (project_shape, policy) read only
// committed content at revision via Git plumbing; the Node host check reads
// only host state; the Compiler check reads worktree content (package.json,
// mise.toml, node_modules/typescript) directly rather than the Git
// snapshot -- these are host-readiness reads of worktree state that setup
// would mutate; they never become analysis input. Other worktree paths are
// inspected only to report their existence, never their content. configPath
// is the --project-config value; an empty string resolves to the default
// "project.json" at revision.
func CheckProjectReadiness(dir, revision, configPath string) (*ReadinessResult, error) {
	policyPath := configPath
	if policyPath == "" {
		policyPath = defaultProjectConfigPath
	}

	// checkPolicy must run before checkProjectShape: the latter needs to know
	// whether a policy validated successfully, and which roots it declared,
	// to recognize a legitimate non-root TypeScript project. Reordering this
	// back would silently make project_shape revert to its root-only
	// heuristic and contradict a passing policy that already names where the
	// project lives.
	policy, roots, err := checkPolicy(dir, revision, policyPath)
	if err != nil {
		return nil, err
	}
	projectShape, err := checkProjectShape(dir, revision, roots, policy.State == ReadinessPass)
	if err != nil {
		return nil, err
	}
	node := checkNodeReadiness()
	compiler := resolveCompiler(dir, roots)
	packageManager := checkPackageManager()

	checks := ReadinessChecks{
		ProjectShape:   projectShape,
		Policy:         policy,
		Node:           node,
		Compiler:       compiler,
		PackageManager: packageManager,
	}

	dirty, err := detectRelevantDirtyWorktree(dir, roots, policyPath)
	if err != nil {
		return nil, err
	}

	status, gaps, nextActions, warnings := aggregateReadiness(checks, dirty.RelevantChanges)

	return &ReadinessResult{
		SchemaVersion: ReadinessSchemaVersion,
		Status:        status,
		Language:      "typescript",
		Revision:      revision,
		DirtyWorktree: dirty,
		Checks:        checks,
		Gaps:          gaps,
		Warnings:      warnings,
		NextActions:   nextActions,
	}, nil
}

// checkPolicy loads and validates the project-config policy file at
// revision, reusing loadProjectConfigForReadiness (project.go) -- the same
// Git-snapshot read/validate path LoadProjectConfig uses, but with a git-
// read failure kept distinct from a content/schema rejection. It returns the
// declared roots on success so the caller can use them for dirty-worktree
// relevance; roots is nil when the policy is missing or invalid. A
// repository-inspection failure is propagated as an error rather than
// reported as policy_missing/policy_invalid: an unreadable repository must
// never produce a confident readiness verdict.
func checkPolicy(dir, revision, policyPath string) (ReadinessCheck, []string, error) {
	exists, err := fileExistsAtRevision(dir, revision, policyPath)
	if err != nil {
		return ReadinessCheck{}, nil, err
	}
	if !exists {
		return ReadinessCheck{State: ReadinessFail, Code: GapPolicyMissing}, nil, nil
	}

	config, err := loadProjectConfigForReadiness(dir, revision, policyPath)
	if err != nil {
		var configErr *ProjectConfigError
		if errors.As(err, &configErr) {
			return ReadinessCheck{State: ReadinessFail, Code: GapPolicyInvalid}, nil, nil
		}
		return ReadinessCheck{}, nil, err
	}
	return ReadinessCheck{State: ReadinessPass}, config.Roots, nil
}

// checkPackageManager always reports not_checked. Real package-manager
// discovery (the package_manager_ambiguous /
// package_manager_config_unverifiable gap codes) is a seam for a later
// task to plug in; fabricating a pass or fail here would misreport a
// check that does not exist yet.
func checkPackageManager() ReadinessCheck {
	return ReadinessCheck{State: ReadinessNotChecked}
}

// ValidateProjectConfigPath validates a --project-config value's shape using
// the same rules LoadProjectConfig enforces, without touching Git or the
// filesystem. --check-project's argument validation calls this before any
// readiness check runs, so an invalid path (absolute, containing "..", or
// using a backslash separator) is rejected at argument time rather than
// surfacing as a false policy_missing/policy_invalid readiness gap.
func ValidateProjectConfigPath(repoPath string) error {
	return validateProjectConfigPath(repoPath)
}
