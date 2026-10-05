package codesignalcli

import (
	"context"

	"os/exec"

	"time"
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
	// ProjectConfigUnreadable means a runGitBytesBoundedWith timeout or
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

// gitOperationalBoundErrorKind distinguishes which of runGitBytesBoundedWith's
// own bounds tripped, so a caller can classify a size-budget failure
// (customer-controlled content) differently from a timeout or stderr-budget
// failure (a resource/environment condition, not content the config author
// can shrink by hand).
type gitOperationalBoundErrorKind int

const (
	gitOperationalBoundTimeout gitOperationalBoundErrorKind = iota
	gitOperationalBoundStdout
	gitOperationalBoundStderr
)

// gitOperationalBoundError marks a runGitBytesBoundedWith failure that comes
// from our own timeout/output-budget enforcement rather than from git's
// stderr. Its message is already complete and safe to surface verbatim to a
// --project-config user: unlike a git failure, it never embeds raw git
// stderr.
type gitOperationalBoundError struct {
	message string
	kind    gitOperationalBoundErrorKind
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

// LoadProjectConfig reads and validates a repository-relative config at an
// immutable Git revision. Reading through Git, rather than the worktree,
// keeps a diff report from mixing committed source facts with uncommitted
// configuration. Git stdout/stderr, wall time, document size, and JSON
// nesting are bounded at this boundary.

// projectConfigGitError classifies a runProjectConfigGit failure into a
// user-facing message that never surfaces raw git stderr. A
// *gitOperationalBoundError is our own timeout/output-budget text and is
// safe to include verbatim. Any other failure means git itself reported the
// read as failed; that case is further split by whether repoPath exists in
// the worktree, so a user who forgot to commit a generated config is told to
// commit it rather than shown a generic not-found message.

// configExistsInWorktree reports whether repoPath is readable in the
// worktree at dir. Any stat failure (not just "does not exist") is treated
// as absent: this function's only caller already falls back to a generic
// not-found-at-revision message in that case, so distinguishing
// permission-denied or other I/O errors from a missing file has no observer.

// runProjectConfigGit is the Git seam used by LoadProjectConfig. Tests may
// replace it to exercise timeout and bound failures without hanging.
var runProjectConfigGit = func(dir string, args ...string) ([]byte, error) {
	return runGitBytesBounded(dir, maxProjectConfigBytes, maxProjectConfigGitStderr, projectConfigGitTimeout, args...)
}

// gitCommandContext builds the git child used by bounded reads. Tests may
// replace it to simulate hung or oversized children.
var gitCommandContext = func(ctx context.Context, dir string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
}

// runGitBytesBounded runs git with a wall-time limit and hard caps on
// collected stdout and stderr, building the child via the package's default
// gitCommandContext seam. The LimitReader stops after maxStdout+1 bytes so
// an oversized blob is detected without buffering the entire child output.

// runGitBytesBoundedWith is the shared bounded-git-read implementation
// behind runGitBytesBounded: a wall-time limit, hard stdout/stderr caps, and
// concurrent pipe draining. buildCmd is the child-construction seam, letting
// callers vary command/environment construction (e.g. project_snapshot.go's
// sanitized-environment snapshot reads) without duplicating this I/O logic.
func runGitBytesBoundedWith(buildCmd func(ctx context.Context, dir string, args ...string) *exec.Cmd, dir string, maxStdout, maxStderr int64, timeout time.Duration, args ...string) ([]byte, error) {
	if err := validateGitReadBounds(timeout, maxStdout, maxStderr); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := buildCmd(ctx, dir, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Drain stdout and stderr concurrently. Sequential reads deadlock when
	// the child fills one pipe's OS buffer while the parent is still
	// blocked reading the other.
	stdoutCh := make(chan gitPipeResult, 1)
	stderrCh := make(chan gitPipeResult, 1)
	go func() {
		stdoutCh <- readGitPipe(stdout, maxStdout+1)
	}()
	go func() {
		stderrCh <- readGitPipe(stderr, maxStderr+1)
	}()
	return finishGitBoundedRead(<-stdoutCh, <-stderrCh, cmd.Wait(), ctx, maxStdout, maxStderr, timeout)
}

// parseProjectConfig performs validateProjectConfigJSON's full decode and
// schema validation, additionally returning the decoded projectConfig. It
// exists so a caller that needs the decoded value (checkPolicy, via
// loadProjectConfigForReadiness) never has to run a second, redundant decode
// of bytes validateProjectConfigJSON already accepted.

// Roots may nest (e.g. "." plus "services/payments"): a multi-module Go
// workspace treats a workspace root and a more specific module root as
// distinct configured roots. Exact duplicate
// identities remain invalid. Layer prefixes below stay non-overlapping
// because they partition policy membership, not discovery roots.

// validateProjectConfigLayers returns the declared layer names so
// validateProjectConfigForbiddenImports and validateProjectConfigCrossFields
// can check their own layer references against it.

// A forbidden_imports entry is an explicit user claim that a layer
// pair exists. Left unchecked, a typo'd from/to that names no
// declared layer would validate cleanly but can never match any
// evaluated (layerFrom, layerTo) pair, silently making that policy
// line a permanent no-op.

// loadProjectConfigForReadiness reads and validates repoPath at revision
// like LoadProjectConfig, but keeps a git-read failure distinct from a
// content/schema rejection instead of collapsing both into
// *ProjectConfigError: checkPolicy must report the former as an
// *OperationalError (exit 1, fail closed) and only the latter as the
// policy_invalid gap. It returns the decoded projectConfig directly so
// checkPolicy needs no second decode of the same bytes.
//
// A stdout-size-budget failure is deliberately classified as content
// rejection (policy_invalid, exit 0) rather than operational (exit 1),
// diverging from LoadProjectConfig's projectConfigGitError, which reports
// every *gitOperationalBoundError -- including this same size-budget case --
// as project_config_invalid at exit 2. The committed policy file's size is
// something its author controls and can fix, unlike a corrupt object store
// or a timed-out git process, so --check-project's read-only, actionable-gap
// contract treats it as a gap rather than an environment failure. A timeout
// or stderr-budget failure still reports *OperationalError: those indicate a
// resource/environment condition, not a defect in the file's content.

// hasDuplicateOrOverlappingPaths reports exact duplicates or ancestor/descendant
// path pairs. Used for layer prefixes, which must partition policy membership.
// Complexity is O(n log n) via sort + adjacent/ancestor checks rather than a
// nested all-pairs scan.

// "." is an ancestor of every other non-empty prefix.

// rejectDuplicateJSONKeys walks one JSON value and rejects duplicate object
// keys and nesting deeper than maxProjectConfigJSONDepth. encoding/json
// otherwise silently keeps the last duplicate value, which would make a
// supposedly frozen config schema depend on parser details.

// walkJSONValue reads and validates the next JSON value from decoder,
// rejecting duplicate object keys and nesting deeper than depth's budget.

// ResolveProjectBackend reports whether a project-analysis backend is
// registered for language. "go" and "typescript" both have registered
// backends today; every other language, including the empty string, remains
// unavailable until its own backend lands.
