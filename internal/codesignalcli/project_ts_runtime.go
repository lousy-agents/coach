package codesignalcli

import (
	"context"
	"errors"
	"fmt"
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
	// NodeExecPath is the absolute, host-resolved path to the exact `node`
	// binary the analyzer is spawned with. It is passed to
	// projectmodel.TSSidecarOptions.BinaryPath directly -- never a bare
	// "node" left for the child's own #!/usr/bin/env node shebang plus a
	// PATH lookup to re-resolve, which cannot guarantee this specific
	// resolved runtime is the one that actually runs.
	NodeExecPath string
	NodeVersion  string
	Kind         string
	Origin       string

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
	CompilerVersion    string
	CompilerOrigin     string
}

const (
	runtimeKindNode   = "node"
	runtimeOriginPath = "path"
)

var errHostNodeNotFound = errors.New("node executable not found on PATH")

const (
	hostNodeVersionProbeTimeout   = 10 * time.Second
	maxHostNodeVersionProbeOutput = 4 << 10
)

// resolveHostNode resolves the exact host `node` executable PrepareTSRuntime
// spawns the analyzer with: its absolute path (exec.LookPath, not a bare
// command name a child's own shebang would independently re-resolve) and
// its raw `node --version` output. This is a separate probe from
// checkNodeReadiness/detectHostNodeMajor (project_readiness.go): readiness
// only needs a major version for --check-project, while runtime preparation
// needs the resolved absolute path itself to spawn against, and the two are
// independent probes by design -- see resolveCompilerForRuntime's doc
// comment for the analogous compiler-side distinction between "readiness
// pass" and "runtime resolvable."
var resolveHostNode = func(ctx context.Context) (execPath, rawVersion string, err error) {
	path, lookErr := exec.LookPath("node")
	if lookErr != nil {
		return "", "", errHostNodeNotFound
	}

	data, exitErr, probeErr := runBoundedSubprocessProbe(ctx, hostNodeVersionProbeTimeout, maxHostNodeVersionProbeOutput, path, "--version")
	switch {
	case errors.Is(probeErr, errBoundedProbeTimedOut):
		return "", "", fmt.Errorf("%s --version timed out", path)
	case probeErr != nil:
		return "", "", probeErr
	case exitErr != nil:
		return "", "", fmt.Errorf("running %s --version: %w", path, exitErr)
	}

	return path, strings.TrimSpace(string(data)), nil
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
// Reaching PrepareTSRuntime with no usable Node or compiler is therefore
// the same class of operational failure as evaluateRevision's NewGoSnapshotFS
// error: the scan cannot start, not a qualified incomplete report.
func PrepareTSRuntime(ctx context.Context, dir string, roots []string) (*tsRuntime, func(), error) {
	nodePath, nodeVersion, err := resolveHostNode(ctx)
	if err != nil {
		return nil, func() {}, fmt.Errorf("coach: resolving host Node runtime for TypeScript analysis: %w", err)
	}

	compiler, err := resolveCompilerForRuntime(dir, roots)
	if err != nil {
		var unresolved *CompilerUnresolvedError
		if errors.As(err, &unresolved) {
			return nil, func() {}, unresolved
		}
		return nil, func() {}, fmt.Errorf("coach: resolving TypeScript compiler for analysis: %w", err)
	}

	analyzerDir, cleanup, err := MaterializeTSAnalyzer(ctx)
	if err != nil {
		return nil, func() {}, fmt.Errorf("coach: materializing private TypeScript analyzer: %w", err)
	}

	return &tsRuntime{
		NodeExecPath:       nodePath,
		NodeVersion:        nodeVersion,
		Kind:               runtimeKindNode,
		Origin:             runtimeOriginPath,
		AnalyzerDir:        analyzerDir,
		AnalyzerShimPath:   filepath.Join(analyzerDir, tsAnalyzerShimAssetPath),
		CompilerModulePath: compiler.Path,
		CompilerVersion:    compiler.Version,
		CompilerOrigin:     compiler.Origin,
	}, cleanup, nil
}
