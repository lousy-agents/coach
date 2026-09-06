package codesignalcli

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	compilerOutcomeEmpty    = "empty"
	compilerOutcomePass     = "pass"
	compilerOutcomeMismatch = "mismatch"
	compilerOutcomeConflict = "conflict"
	compilerOutcomeRejected = "rejected"
)

const (
	compilerOriginProject             = "project"
	compilerOriginMiseProject         = "mise_project"
	compilerOriginMiseGlobal          = "mise_global"
	compilerDeclarationOriginManifest = "manifest"
)

// SupportedTypescriptVersions is the compiled-in set of exact TypeScript
// compiler versions this build will load. TypeScript does not follow
// semantic versioning and the analyzer depends on typescript/unstable/*
// subpaths, so no range is promised. The set is stored ascending; the
// newest member is the expected_version on mismatch and missing gaps.
var SupportedTypescriptVersions = []string{"7.0.2"}

func newestSupportedTypescriptVersion() string {
	return SupportedTypescriptVersions[len(SupportedTypescriptVersions)-1]
}

func isSupportedTypescriptVersion(version string) bool {
	for _, candidate := range SupportedTypescriptVersions {
		if candidate == version {
			return true
		}
	}
	return false
}

func supportedTypescriptVersionsCopy() []string {
	out := make([]string, len(SupportedTypescriptVersions))
	copy(out, SupportedTypescriptVersions)
	return out
}

// compilerOriginOutcome is one origin's contribution to resolveCompiler.
// Only compilerOutcomeEmpty (no candidate at all) lets resolution continue
// to the next, lower-precedence origin. Every other outcome is terminal:
// compilerOutcomeConflict (multiple plausible exact candidates within this
// origin, with no expected/found relationship between them) and
// compilerOutcomeRejected (a candidate manifest/lockfile existed but could
// not be read or parsed) are reported directly rather than silently falling
// through, per the frozen ambiguity rule.
//
// origin identifies which resolution origin produced a compilerOutcomePass
// ("project", "mise_project", "mise_global"); it is empty for every other
// state. resolveCompiler (the ReadinessCheck projection) never reads it --
// only resolveCompilerForRuntime does, to decide how to locate the winning
// origin's compiler on disk.
//
// manifestDir is the package.json directory the project origin resolved
// (nearest at or above a selected policy root, bounded by the worktree).
// locateCompilerForOrigin reads node_modules/typescript from this directory,
// never blindly from the worktree top. It is empty for every non-project
// origin and every non-pass state.
type compilerOriginOutcome struct {
	state        string
	version      string
	expected     string
	found        string
	origin       string
	manifestDir  string
	rootFindings []ReadinessRootFinding
	declared     string
	warnDecl     bool
}

var exactVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$`)

func isExactVersion(value string) bool {
	return exactVersionPattern.MatchString(strings.TrimSpace(value))
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

// resolveCompiler applies the frozen compiler-resolution order: a unique
// exact compiler from the project manifest context, then project mise
// configuration, then global mise configuration. dir may be any directory
// inside the Git worktree (checkProjectShape/checkPolicy already tolerate
// this by resolving paths through Git plumbing); compilerWorktreeRoot(dir)
// is the walk ceiling / repository bound, not the project-manifest origin.
// The project-manifest origin is per selected policy root: nearest
// package.json at or above that root, stopping at the worktree top. An
// empty roots list (missing/invalid policy) is equivalent to roots: ["."]
// -- worktree-top only, never a walk down to auto-discover nested
// manifests. Mise origins stay rooted at the worktree top (frozen order
// unchanged). These are host-readiness reads of the worktree (the manifest
// that setup would mutate and the node_modules the compiler lives in are
// worktree state), never the analyzed Git revision.
func resolveCompiler(dir string, roots []string) ReadinessCheck {
	root := compilerWorktreeRoot(dir)
	var pendingDecl string
	var pendingFound string
	for _, resolve := range compilerOrigins(root, roots) {
		outcome := resolve()
		pendingDecl = firstPendingDecl(pendingDecl, outcome)
		if outcome.state == compilerOutcomeEmpty {
			pendingFound = firstNonEmpty(pendingFound, outcome.found)
			continue
		}
		return compilerCheckFromNonEmptyOrigin(outcome, pendingDecl, pendingFound)
	}
	return missingCompilerCheck(pendingFound, pendingDecl)
}

// compilerOrigins returns the frozen resolution order's three origin
// probes: project manifest (per selected policy root), then project mise
// at the worktree top, then global mise. resolveCompiler and
// resolveCompilerForRuntime both walk this same list, stopping at the
// first non-empty outcome, so the order is defined exactly once.
func compilerOrigins(root string, roots []string) []func() compilerOriginOutcome {
	return []func() compilerOriginOutcome{
		func() compilerOriginOutcome { return resolveProjectManifestCompiler(root, roots) },
		func() compilerOriginOutcome { return resolveMiseProjectCompiler(root) },
		func() compilerOriginOutcome { return resolveMiseGlobalCompiler() },
	}
}

// compilerWorktreeRootTimeout and maxCompilerWorktreeRootOutput bound the
// `git rev-parse --show-toplevel` call compilerWorktreeRoot makes, mirroring
// this package's other bounded git reads.
const (
	compilerWorktreeRootTimeout   = 10 * time.Second
	maxCompilerWorktreeRootOutput = 4 << 10
	maxCompilerWorktreeRootStderr = 4 << 10
)

// compilerWorktreeRoot resolves dir to its enclosing Git worktree's
// top-level directory via `git rev-parse --show-toplevel`, reusing
// runGitBytesBounded (project.go). This is the walk ceiling for nearest-
// package.json resolution, not the project-manifest origin: a nested
// invocation cwd must not walk above the worktree. By the time
// resolveCompiler runs, CheckProjectReadiness has already verified dir is
// inside a readable Git worktree (checkPolicy/checkProjectShape would have
// failed closed otherwise), so a failure here falls back to dir itself
// rather than blocking the compiler check on a resolution step expected
// to succeed.
func compilerWorktreeRoot(dir string) string {
	output, err := runGitBytesBounded(dir, maxCompilerWorktreeRootOutput, maxCompilerWorktreeRootStderr, compilerWorktreeRootTimeout, "rev-parse", "--show-toplevel")
	if err != nil {
		return dir
	}
	root := strings.TrimSpace(string(output))
	if root == "" {
		return dir
	}
	return root
}

// compilerRuntimeResolution is resolveCompilerForRuntime's success result: a
// genuinely locatable exact compiler, identified by absolute filesystem
// path, ready for PrepareTSRuntime to spawn the analyzer against via
// --compiler-module.
type compilerRuntimeResolution struct {
	Origin            string
	Version           string
	Path              string
	NativePackagePath string
}

// CompilerUnresolvedError is the typed scan-time failure when analysis
// cannot start because a required compiler or host Node runtime is
// unresolved. The CLI maps it to exit 2 with one stderr remediation line
// naming the gap code and the exact --check-project invocation; it is
// never wrapped with a "coach:" prefix. Probe timeouts stay operational
// errors, not this type.
type CompilerUnresolvedError struct {
	Code       string
	ConfigPath string
}

func (e *CompilerUnresolvedError) Error() string {
	return e.RemediationLine()
}

func (e *CompilerUnresolvedError) RemediationLine() string {
	invocation := "coach codesignal --baseline --check-project --project-language typescript"
	if e.ConfigPath != "" {
		invocation += " --project-config " + e.ConfigPath
	}
	return e.Code + ": run " + invocation
}

func compilerUnresolved(code string) *CompilerUnresolvedError {
	return &CompilerUnresolvedError{Code: code}
}

// resolveCompilerForRuntime resolves the same frozen origin precedence as
// resolveCompiler (project manifest per selected policy root, then project
// mise, then global mise) and requires the winning origin's compiler to be
// genuinely present at an absolute filesystem location. roots is the same
// selected-root list CheckProjectReadiness passes to resolveCompiler; dir
// is the walk ceiling only. A declared-but-not-installed origin is
// terminal -- the same typescript_compiler_missing outcome resolveCompiler
// reports -- never a fall-through to a lower-precedence origin that happens
// to be on disk. Falling through would select a different compiler than
// --check-project reported, which the frozen origin rule forbids.
func resolveCompilerForRuntime(dir string, roots []string) (compilerRuntimeResolution, error) {
	root := compilerWorktreeRoot(dir)
	for _, resolve := range compilerOrigins(root, roots) {
		outcome := resolve()
		if outcome.state == compilerOutcomeEmpty {
			continue
		}
		return runtimeResolutionFromOrigin(outcome)
	}
	return compilerRuntimeResolution{}, compilerUnresolved(GapTypescriptCompilerMissing)
}

// locateCompilerForOrigin resolves outcome's absolute on-disk compiler
// package location for the origin that produced it. Returns ok=false when
// the origin only declared a version without a genuinely installed
// compiler. Callers treat that as terminal (readiness:
// typescript_compiler_missing; runtime: CompilerUnresolvedError), never as
// an empty origin that may fall through.
func locateCompilerForOrigin(outcome compilerOriginOutcome) (string, bool) {
	switch outcome.origin {
	case compilerOriginProject:
		return locateProjectCompiler(outcome.manifestDir)
	case compilerOriginMiseProject, compilerOriginMiseGlobal:
		return locateMiseTypescriptInstall(context.Background(), outcome.version)
	default:
		return "", false
	}
}

func locateProjectCompiler(manifestDir string) (string, bool) {
	if manifestDir == "" {
		return "", false
	}
	_, exists, unreadable := readInstalledTypescriptVersion(manifestDir)
	if unreadable || !exists {
		return "", false
	}
	return filepath.Join(manifestDir, "node_modules", "typescript"), true
}
