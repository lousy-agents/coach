package codesignalcli

import (
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// Project-config boundary budgets. Config is repository-controlled input and
// must fail closed before unbounded memory/CPU or a hung git child can stall
// the CLI.
const (
	maxProjectConfigBytes     = 1 << 20 // 1 MiB
	maxProjectConfigJSONDepth = 32
	maxProjectConfigGitStderr = 64 << 10 // 64 KiB
	projectConfigGitTimeout   = 30 * time.Second
	// maxProjectConfigLayerPrefixes bounds the sorted prefix-overlap scan so a
	// hostile but still ≤1 MiB config cannot force quadratic validation CPU.
	maxProjectConfigLayerPrefixes = 4096
	// maxProjectConfigRoots bounds the declared roots list. Unlike layer
	// prefixes (a pure in-process string-comparison budget), each declared
	// root can drive up to three git child-process spawns in
	// checkProjectShape's non-root package.json probe
	// (project_readiness.go), so this budget must stay small enough that
	// even the worst case (no package.json under any root) completes in a
	// few seconds rather than fanning out into tens of thousands of git
	// invocations from a config that is still well under
	// maxProjectConfigBytes.
	maxProjectConfigRoots = 256
)

// ProjectConfigErrorKind discriminates why a *ProjectConfigError was
// produced, distinct from its human-facing Message (which stays frozen). A
// caller that needs to react differently to "no policy was ever committed"
// than to a usage error, an uncommitted-in-worktree file, or a transient git
// failure reads Kind rather than pattern-matching Message.
type ProjectConfigErrorKind int

const (
	// ProjectConfigKindUnset is the zero value: a *ProjectConfigError whose
	// cause was never classified. It is deliberately non-actionable, so an
	// untagged construction site cannot alias a specific kind.
	ProjectConfigKindUnset ProjectConfigErrorKind = iota
	// ProjectConfigNotFound means repoPath does not exist at revision and is
	// not present in the worktree either: no policy was ever authored or
	// committed.
	ProjectConfigNotFound
	// ProjectConfigInvalid means repoPath itself failed shape validation, or
	// content read at revision failed to parse or validate against the
	// schema.
	ProjectConfigInvalid
	// ProjectConfigUncommitted means repoPath exists in the worktree but not
	// at the analyzed revision.
	ProjectConfigUncommitted
	// ProjectConfigUnreadable means a gitrepo.RunBytesBoundedWith timeout or
	// output-budget bound tripped while reading repoPath: a transient
	// operational condition, not a defect in the file's content.
	ProjectConfigUnreadable
)

// ProjectConfigError signals a --project-config value that is missing,
// unreadable, or does not satisfy the frozen v1 schema. It maps to exit code
// 2 and is reported as a single stderr message; no report is written to
// stdout.
type ProjectConfigError struct {
	Message string
	Kind    ProjectConfigErrorKind
}

// ProjectBackendUnavailableError signals a valid project configuration whose
// requested language has no registered project-analysis backend. It maps to
// exit code 3 and is reported in the local CodeSignal document.
type ProjectBackendUnavailableError struct {
	Message string
}

type projectConfig struct {
	SchemaVersion    string                   `json:"schema_version"`
	Roots            []string                 `json:"roots"`
	Layers           []projectConfigLayer     `json:"layers,omitempty"`
	ForbiddenImports []projectForbiddenImport `json:"forbidden_imports,omitempty"`
	SourceSinkPack   string                   `json:"source_sink_pack,omitempty"`
	RequiredLayer    string                   `json:"required_layer,omitempty"`
}

type projectConfigLayer struct {
	Name     string   `json:"name"`
	Prefixes []string `json:"prefixes"`
}

type projectForbiddenImport struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// runProjectConfigGit is the Git seam used by LoadProjectConfig. Tests may
// replace it to exercise timeout and bound failures without hanging.
var runProjectConfigGit = func(dir string, args ...string) ([]byte, error) {
	return gitrepo.RunBytesBounded(dir, maxProjectConfigBytes, maxProjectConfigGitStderr, projectConfigGitTimeout, args...)
}

func (e *ProjectConfigError) Error() string { return e.Message }

func (e *ProjectBackendUnavailableError) Error() string { return e.Message }
