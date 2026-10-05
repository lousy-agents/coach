package codesignalcli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
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

func mapHostNodeResolveError(err error) error {
	if errors.Is(err, tstoolchain.ErrHostNodeNotFound) {
		return &tstoolchain.RuntimeUnresolvedError{Code: projectreadiness.GapNodeMissing}
	}
	if errors.Is(err, tstoolchain.ErrHostNodeMajorDisallowed) {
		return &tstoolchain.RuntimeUnresolvedError{Code: projectreadiness.GapNodeUnsupported}
	}
	return fmt.Errorf("coach: resolving host Node runtime for TypeScript analysis: %w", err)
}

func mapCompilerRuntimeError(err error) error {
	var unresolved *tstoolchain.CompilerUnresolvedError
	if errors.As(err, &unresolved) {
		return unresolved
	}
	return fmt.Errorf("coach: resolving TypeScript compiler for analysis: %w", err)
}

// PrepareTSRuntime resolves and materializes everything tsProjectBackend
// needs to run one --project-language typescript analysis confined to a
// private, host-approved runtime: the exact host Node executable, the exact
// approved TypeScript compiler (see tstoolchain.ResolveCompilerForRuntime), and a
// freshly materialized private analyzer directory (MaterializeTSAnalyzer).
// On success, the caller owns the returned cleanup and must call it,
// exactly like MaterializeTSAnalyzer's own contract -- it tears down the
// materialized analyzer directory and is safe to call more than once.
//
// dir is the walk ceiling (repository root); roots are the selected
// policy roots threaded into tstoolchain.ResolveCompilerForRuntime so readiness and
// Analyze share one root-scoped project-manifest origin. Compiler-
// resolution reads are host-readiness reads of the analyzed repository's
// worktree (its package.json/mise.toml/node_modules), never analysis input
// -- the same distinction projectcheck.Run's own tstoolchain.ResolveCompiler
// call documents.
//
// Every failure here is fatal (a non-nil error, cleanup a no-op), never a
// soft projectmodel.DiagBackendUnavailable degrade. Analyze does not run
// --check-project; the two share tstoolchain.ResolveCompiler's locatable-PASS rule so a
// readiness-green repository cannot reach an unlocatable compiler here.
// A clean miss of host Node or of a supported compiler is a
// tstoolchain.CompilerUnresolvedError (exit 2). Other probe failures stay operational.
func PrepareTSRuntime(ctx context.Context, dir string, roots []string) (*tsRuntime, func(), error) {
	nodePath, nodeVersion, err := tstoolchain.ResolveHostNode(ctx)
	if err != nil {
		return nil, func() {}, mapHostNodeResolveError(err)
	}

	compiler, err := tstoolchain.ResolveCompilerForRuntime(dir, roots)
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
