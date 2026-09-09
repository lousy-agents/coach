package codesignalcli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// tsRuntime is the prepared runtime descriptor: everything tsProjectBackend
// needs to spawn the private TypeScript analyzer confined to one approved
// compiler and its own materialized directory, without resolving an
// analyzer, a compiler, or a runtime from the analyzed repository itself.
type tsRuntime struct {
	ExecPath string
	ExecArgs []string
	Version  string
	Kind     string
	Origin   string

	// AnalyzerDir is the materialized private analyzer directory
	// (see MaterializeTSAnalyzer). The analyzer process's working directory
	// is this path, not the analyzed repository.
	AnalyzerDir string

	// AnalyzerShimPath is the absolute path to the materialized private
	// analyzer entrypoint (see MaterializeTSAnalyzer), always node's first
	// argv.
	AnalyzerShimPath string

	// CompilerModulePath is the absolute filesystem path to the resolved,
	// approved TypeScript compiler package root, passed to the analyzer as
	// --compiler-module.
	CompilerModulePath string
	NativePackagePath  string
	CompilerVersion    string
	CompilerOrigin     string
}

const (
	runtimeKindNode   = "node"
	runtimeOriginPath = "path"
)

var (
	errHostNodeNotFound        = errors.New("node executable not found on PATH")
	errHostNodeMajorDisallowed = errors.New("host node major is outside the analysis runtime set")
)

const (
	hostNodeVersionProbeTimeout   = 10 * time.Second
	maxHostNodeVersionProbeOutput = 4 << 10
)

func analysisNodeMajorAllowed(major int) bool {
	return major == 24 || major == 26
}

func hostNodeProbeEnv() []string {
	return []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + os.Getenv("HOME"),
	}
}

func mapHostNodeProbeError(path, probe string, exitErr, probeErr error) error {
	switch {
	case errors.Is(probeErr, errBoundedProbeTimedOut):
		return fmt.Errorf("%s %s timed out", path, probe)
	case probeErr != nil:
		return probeErr
	case exitErr != nil:
		return fmt.Errorf("running %s %s: %w", path, probe, exitErr)
	default:
		return nil
	}
}

// resolveHostNode resolves the exact host `node` executable PrepareTSRuntime
// spawns the analyzer with: process.execPath from a probe of the LookPath
// result (never a version-manager shim) and its raw `node --version` output.
// This is a separate probe from checkNodeReadiness/detectHostNodeMajor
// (project_readiness.go): readiness only needs a major version for
// --check-project, while runtime preparation needs the resolved absolute
// path itself to spawn against, and the two are independent probes by
// design -- see resolveCompilerForRuntime's doc comment for the analogous
// compiler-side distinction between "readiness pass" and "runtime
// resolvable."
var resolveHostNode = func(ctx context.Context) (execPath, rawVersion string, err error) {
	path, lookErr := exec.LookPath("node")
	if lookErr != nil {
		return "", "", errHostNodeNotFound
	}

	data, exitErr, probeErr := runBoundedSubprocessProbeAt(ctx, hostNodeVersionProbeTimeout, maxHostNodeVersionProbeOutput, "", hostNodeProbeEnv(), path, "--version")
	if err := mapHostNodeProbeError(path, "--version", exitErr, probeErr); err != nil {
		return "", "", err
	}

	rawVersion = strings.TrimSpace(string(data))
	major, parseErr := parseNodeMajor(rawVersion)
	if parseErr != nil {
		return "", "", parseErr
	}
	if !analysisNodeMajorAllowed(major) {
		return "", "", errHostNodeMajorDisallowed
	}

	execData, execExitErr, execProbeErr := runBoundedSubprocessProbeAt(ctx, hostNodeVersionProbeTimeout, maxHostNodeVersionProbeOutput, "", hostNodeProbeEnv(), path, "-p", "process.execPath")
	if err := mapHostNodeProbeError(path, "process.execPath probe", execExitErr, execProbeErr); err != nil {
		return "", "", err
	}

	execPath = strings.TrimSpace(string(execData))
	if !filepath.IsAbs(execPath) {
		return "", "", fmt.Errorf("host node process.execPath is not absolute: %q", execPath)
	}
	return execPath, rawVersion, nil
}

func mapHostNodeResolveError(err error) error {
	if errors.Is(err, errHostNodeNotFound) {
		return compilerUnresolved(GapNodeMissing)
	}
	if errors.Is(err, errHostNodeMajorDisallowed) {
		return compilerUnresolved(GapNodeUnsupported)
	}
	return fmt.Errorf("coach: resolving host Node runtime for TypeScript analysis: %w", err)
}

func mapCompilerRuntimeError(err error) error {
	var unresolved *CompilerUnresolvedError
	if errors.As(err, &unresolved) {
		return unresolved
	}
	return fmt.Errorf("coach: resolving TypeScript compiler for analysis: %w", err)
}

// PrepareTSRuntime resolves and materializes everything tsProjectBackend
// needs to run one --project-language typescript analysis confined to a
// private, host-approved runtime: the exact host Node executable, the exact
// approved TypeScript compiler (see resolveCompilerForRuntime), and a
// freshly materialized private analyzer directory (MaterializeTSAnalyzer).
// On success, the caller owns the returned cleanup and must call it,
// exactly like MaterializeTSAnalyzer's own contract -- it tears down the
// materialized analyzer directory and is safe to call more than once.
//
// dir is the walk ceiling (repository root); roots are the selected
// policy roots threaded into resolveCompilerForRuntime so readiness and
// Analyze share one root-scoped project-manifest origin. Compiler-
// resolution reads are host-readiness reads of the analyzed repository's
// worktree (its package.json/mise.toml/node_modules), never analysis input
// -- the same distinction CheckProjectReadiness's own resolveCompiler
// call documents.
//
// Every failure here is fatal (a non-nil error, cleanup a no-op), never a
// soft projectmodel.DiagBackendUnavailable degrade. Analyze does not run
// --check-project; the two share resolveCompiler's locatable-PASS rule so a
// readiness-green repository cannot reach an unlocatable compiler here.
// A clean miss of host Node or of a supported compiler is a
// CompilerUnresolvedError (exit 2). Other probe failures stay operational.
func PrepareTSRuntime(ctx context.Context, dir string, roots []string) (*tsRuntime, func(), error) {
	nodePath, nodeVersion, err := resolveHostNode(ctx)
	if err != nil {
		return nil, func() {}, mapHostNodeResolveError(err)
	}

	compiler, err := resolveCompilerForRuntime(dir, roots)
	if err != nil {
		return nil, func() {}, mapCompilerRuntimeError(err)
	}

	analyzerDir, cleanup, err := MaterializeTSAnalyzer(ctx)
	if err != nil {
		return nil, func() {}, fmt.Errorf("coach: materializing private TypeScript analyzer: %w", err)
	}

	shim := filepath.Join(analyzerDir, tsAnalyzerShimAssetPath)
	execArgs := []string{
		shim,
		"--compiler-module=" + compiler.Path,
		"--native-package=" + compiler.NativePackagePath,
	}
	return &tsRuntime{
		ExecPath:           nodePath,
		ExecArgs:           execArgs,
		Version:            nodeVersion,
		Kind:               runtimeKindNode,
		Origin:             runtimeOriginPath,
		AnalyzerDir:        analyzerDir,
		AnalyzerShimPath:   shim,
		CompilerModulePath: compiler.Path,
		NativePackagePath:  compiler.NativePackagePath,
		CompilerVersion:    compiler.Version,
		CompilerOrigin:     compiler.Origin,
	}, cleanup, nil
}
