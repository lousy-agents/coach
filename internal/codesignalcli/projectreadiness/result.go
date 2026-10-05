// Package projectreadiness is the --check-project result model: the checks,
// gaps, warnings, and next actions a readiness run reports, and the gap-code
// table that fixes each gap's status and remediation kind.
package projectreadiness

const (
	SchemaVersion = "1"
)

type Status string

const (
	StatusOutsideSupport    Status = "outside_support"
	StatusNeedsPrerequisite Status = "needs_prerequisite"
	StatusNeedsPolicy       Status = "needs_policy"
	StatusReadyWithLimits   Status = "ready_with_limits"
	StatusReady             Status = "ready"
)

// Gap names one gap code. PackageManagerKind discriminates between
// independent package_manager_* findings that may coexist in gaps[]: a
// project-adapter kind ("npm"/"pnpm"/"bun"/"yarn", checks.package_manager)
// or a mise setup-choice kind ("mise_project"/"mise_global"), which never
// occupies checks.package_manager. It is empty for every other gap code.
type Gap struct {
	Code               string `json:"code"`
	PackageManagerKind string `json:"package_manager_kind,omitempty"`
}

type Warning struct {
	Code              string `json:"code"`
	DeclaredVersion   string `json:"declared_version,omitempty"`
	FoundVersion      string `json:"found_version,omitempty"`
	DeclarationOrigin string `json:"declaration_origin,omitempty"`
	Root              string `json:"root,omitempty"`
}

// NextAction is one remediation entry. PackageManagerKind names
// which installation choice a resolve_package_manager entry is about (see
// Gap); a rejected adapter and a rejected mise origin surface as
// two distinctly keyed entries rather than colliding into one. Choices
// lists the still-verified installation-choice kinds a prepare_compiler
// entry may use once a package_manager_* finding has withheld another one;
// it is nil when no package_manager_* finding applies.
type NextAction struct {
	Kind               string   `json:"kind"`
	Executable         bool     `json:"executable"`
	RuntimeKind        string   `json:"runtime_kind,omitempty"`
	PackageManagerKind string   `json:"package_manager_kind,omitempty"`
	Supported          []string `json:"supported,omitempty"`
	FoundVersion       string   `json:"found_version,omitempty"`
	Detail             string   `json:"detail,omitempty"`
	Choices            []string `json:"choices,omitempty"`
}

type DirtyWorktree struct {
	RelevantChanges bool     `json:"relevant_changes"`
	Paths           []string `json:"paths"`
}

type Result struct {
	SchemaVersion string        `json:"schema_version"`
	Status        Status        `json:"status"`
	Language      string        `json:"language"`
	Revision      string        `json:"revision"`
	DirtyWorktree DirtyWorktree `json:"dirty_worktree"`
	Checks        Checks        `json:"checks"`
	Gaps          []Gap         `json:"gaps"`
	Warnings      []Warning     `json:"warnings"`
	NextActions   []NextAction  `json:"next_actions"`

	// MiseChoices is the verified mise setup choices this run computed
	// (evaluateMiseSetupChoices, owner decision D5). It never serializes:
	// AvailableSetupChoices reads it so the menu and the readiness gaps are
	// the same computation rather than two that can disagree.
	MiseChoices []MiseChoice `json:"-"`
}

// MiseChoice is one mise-origin setup-choice input to
// aggregateReadiness. Kind distinguishes "mise_project" from "mise_global"
// so the two configuration scopes stay independently verifiable and
// withholdable. Verified means mise itself resolved a supported,
// hazard-free compiler-install origin at Kind; when Verified is false, Code
// names the package_manager_* gap that rejects this specific choice,
// without touching any other choice. Reason is what AvailableSetupChoices
// tells a customer when it withholds this scope from the setup menu, and is
// deliberately not Code: a trusted scope that simply pins nothing installable
// is withheld with no gap code at all, which Code alone cannot express.
type MiseChoice struct {
	Kind     string
	Verified bool
	Code     string
	Reason   string
}

const (
	ReasonMiseUnconfigured = "mise_unconfigured"
	ReasonMiseUnverifiable = "mise_unverifiable"
)
