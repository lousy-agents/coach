package codesignalcli

import (
	"errors"
	"strconv"
)

const (
	ReadinessSchemaVersion   = "1"
	defaultProjectConfigPath = "project.json"
)

var SupportedNodeMajors = []int{24, 26}

func nodeMajorSupported(major int) bool {
	for _, supported := range SupportedNodeMajors {
		if supported == major {
			return true
		}
	}
	return false
}

func supportedNodeMajorsCopy() []string {
	out := make([]string, len(SupportedNodeMajors))
	for i, major := range SupportedNodeMajors {
		out[i] = strconv.Itoa(major)
	}
	return out
}

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

const (
	GapUnsupportedRepositoryShape        = "unsupported_repository_shape"
	GapNodeMissing                       = "node_missing"
	GapNodeUnsupported                   = "node_unsupported"
	GapNodeUnverifiable                  = "node_unverifiable"
	GapTypescriptCompilerMissing         = "typescript_compiler_missing"
	GapTypescriptVersionMismatch         = "typescript_version_mismatch"
	GapTypescriptVersionConflict         = "typescript_version_conflict"
	GapPackageManagerAmbiguous           = "package_manager_ambiguous"
	GapPackageManagerVersionUnverifiable = "package_manager_version_unverifiable"
	GapPackageManagerVersionUnsupported  = "package_manager_version_unsupported"
	GapPackageManagerConfigUnverifiable  = "package_manager_config_unverifiable"
	GapPolicyMissing                     = "policy_missing"
	GapPolicyInvalid                     = "policy_invalid"
)

// WarnCompilerDeclarationMismatch is the limit-class warning when a
// selected root's manifest declares a typescript version other than the
// compiler the scan will use. A winning non-project origin warns for any
// differing declaration (range or exact); a winning project origin warns
// only for a stale exact pin -- a range is never warned about, and range
// satisfaction is never evaluated. It never appears in gaps[] and carries
// no next action: Coach never edits manifests.
const WarnCompilerDeclarationMismatch = "compiler_declaration_mismatch"

type ReadinessRootFinding struct {
	Root    string `json:"root"`
	Version string `json:"version,omitempty"`
}

type ReadinessOriginFinding struct {
	Origin string
	Class  string
}

type ReadinessDeclarationMismatch struct {
	Root     string
	Declared string
}

type ReadinessCheck struct {
	State             ReadinessState           `json:"state"`
	Code              string                   `json:"code,omitempty"`
	Kind              string                   `json:"kind,omitempty"`
	Version           string                   `json:"version,omitempty"`
	ExpectedVersion   string                   `json:"expected_version,omitempty"`
	FoundVersion      string                   `json:"found_version,omitempty"`
	SupportedVersions []string                 `json:"supported_versions,omitempty"`
	RootFindings      []ReadinessRootFinding   `json:"root_findings,omitempty"`
	Origin            string                   `json:"origin,omitempty"`
	Detail            string                   `json:"detail,omitempty"`
	DeclaredVersion   string                   `json:"-"`
	DeclarationOrigin string                   `json:"-"`
	OriginFindings    []ReadinessOriginFinding `json:"-"`

	DeclarationMismatches []ReadinessDeclarationMismatch `json:"-"`
}

type ReadinessChecks struct {
	ProjectShape   ReadinessCheck `json:"project_shape"`
	Policy         ReadinessCheck `json:"policy"`
	Node           ReadinessCheck `json:"node"`
	Runtime        ReadinessCheck `json:"runtime"`
	Compiler       ReadinessCheck `json:"compiler"`
	PackageManager ReadinessCheck `json:"package_manager"`
}

// ReadinessGap names one gap code. PackageManagerKind discriminates between
// independent package_manager_* findings that may coexist in gaps[]: a
// project-adapter kind ("npm"/"pnpm"/"bun"/"yarn", checks.package_manager)
// or a mise setup-choice kind ("mise_project"/"mise_global"), which never
// occupies checks.package_manager. It is empty for every other gap code.
type ReadinessGap struct {
	Code               string `json:"code"`
	PackageManagerKind string `json:"package_manager_kind,omitempty"`
}

type ReadinessWarning struct {
	Code              string `json:"code"`
	DeclaredVersion   string `json:"declared_version,omitempty"`
	FoundVersion      string `json:"found_version,omitempty"`
	DeclarationOrigin string `json:"declaration_origin,omitempty"`
	Root              string `json:"root,omitempty"`
}

// ReadinessNextAction is one remediation entry. PackageManagerKind names
// which installation choice a resolve_package_manager entry is about (see
// ReadinessGap); a rejected adapter and a rejected mise origin surface as
// two distinctly keyed entries rather than colliding into one. Choices
// lists the still-verified installation-choice kinds a prepare_compiler
// entry may use once a package_manager_* finding has withheld another one;
// it is nil when no package_manager_* finding applies.
type ReadinessNextAction struct {
	Kind               string   `json:"kind"`
	Executable         bool     `json:"executable"`
	RuntimeKind        string   `json:"runtime_kind,omitempty"`
	Supported          []string `json:"supported,omitempty"`
	FoundVersion       string   `json:"found_version,omitempty"`
	Detail             string   `json:"detail,omitempty"`
	PackageManagerKind string   `json:"package_manager_kind,omitempty"`
	Choices            []string `json:"choices,omitempty"`
}

type ReadinessDirtyWorktree struct {
	RelevantChanges bool     `json:"relevant_changes"`
	Paths           []string `json:"paths"`
}

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

func CheckProjectReadiness(dir, revision, configPath string) (*ReadinessResult, error) {
	policyPath := configPath
	if policyPath == "" {
		policyPath = defaultProjectConfigPath
	}

	policy, roots, err := checkPolicy(dir, revision, policyPath)
	if err != nil {
		return nil, err
	}
	projectShape, err := checkProjectShape(dir, revision, roots, policy.State == ReadinessPass)
	if err != nil {
		return nil, err
	}
	runtime := checkNodeReadiness()
	node := nodeCompatibilityMirror(runtime)
	compiler := resolveCompiler(dir, roots)
	packageManager, err := checkPackageManager(dir, revision)
	if err != nil {
		return nil, err
	}

	checks := ReadinessChecks{
		ProjectShape:   projectShape,
		Policy:         policy,
		Node:           node,
		Runtime:        runtime,
		Compiler:       compiler,
		PackageManager: packageManager,
	}

	dirty, err := detectRelevantDirtyWorktree(dir, roots, policyPath)
	if err != nil {
		return nil, err
	}

	status, gaps, nextActions, warnings := aggregateReadiness(checks, dirty.RelevantChanges, evaluateMiseSetupChoices(dir, roots))

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

// packageManagerKindYarn has no supported-version row in the adapter
// matrix, so any Yarn metadata at revision unconditionally reports
// package_manager_version_unsupported regardless of which Yarn version is
// actually in use.
const packageManagerKindYarn = "yarn"

// checkPackageManager only checks for a committed yarn.lock. This is
// deliberate, not a partial implementation to finish here: detecting and
// version-checking npm/pnpm/Bun adapters is a separate task's scope, so
// every other repository shape stays not_checked.
func checkPackageManager(dir, revision string) (ReadinessCheck, error) {
	yarnLock, err := fileExistsAtRevision(dir, revision, "yarn.lock")
	if err != nil {
		return ReadinessCheck{}, err
	}
	if yarnLock {
		return ReadinessCheck{State: ReadinessFail, Code: GapPackageManagerVersionUnsupported, Kind: packageManagerKindYarn}, nil
	}
	return ReadinessCheck{State: ReadinessNotChecked}, nil
}

func ValidateProjectConfigPath(repoPath string) error {
	return validateProjectConfigPath(repoPath)
}
