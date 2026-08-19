package projectmodel

import (
	"context"
	"fmt"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// CallGraphAlgorithm identifies the pinned static-analysis backend
// BuildGoCallGraph uses to resolve direct calls, so downstream consumers can
// attribute coverage/precision claims to the exact method and x/tools
// version that produced a CallGraphResult's facts. Direct-call resolution
// uses the same StaticCallee()/IsInvoke() definition
// golang.org/x/tools/go/callgraph/static.CallGraph uses; that package is
// also invoked directly, and its node count is recorded in
// Coverage.Counts["callgraph_static_nodes"] as a cross-check. The version
// suffix is pinned to go.mod's golang.org/x/tools requirement; see
// TestCallGraphAlgorithmVersionMatchesGoMod, which fails if they drift.
const CallGraphAlgorithm = "go-callgraph-static@1+golang.org/x/tools@v0.49.0"

// Stable call-site diagnostic codes for CallGraphResult.Coverage.Diagnostics[i].Code.
const (
	// DiagCallUnresolvedInterface marks an interface-method call site: the
	// concrete receiver is unknown statically, so the site contributes no
	// CallFact.
	DiagCallUnresolvedInterface = "project_call_unresolved_interface"
	// DiagCallUnresolvedFunctionValue marks a call through a func-typed
	// variable, field, or parameter with no statically known callee.
	DiagCallUnresolvedFunctionValue = "project_call_unresolved_function_value"
	// DiagCallUnresolvedReflection marks a call dispatched through
	// reflect.Value.Call/CallSlice: SSA resolves the callee to that
	// reflect method itself, not to the reflected target, so the site
	// contributes no CallFact.
	DiagCallUnresolvedReflection = "project_call_unresolved_reflection"
	// DiagCallUnresolvedFrameworkRegistration marks a function/handler
	// value passed to a well-known HTTP registration call (net/http's
	// Handle/HandleFunc or (*http.ServeMux).Handle/HandleFunc) rather than
	// invoked directly: the framework invokes it later, at a call site
	// static analysis cannot see.
	DiagCallUnresolvedFrameworkRegistration = "project_call_unresolved_framework_registration"
	// DiagCallUnresolvedSyntheticWrapper marks a call site whose resolved
	// callee is itself a synthetic wrapper function (fn.Pkg == nil: a
	// bound-method value or a promoted/embedded-method thunk) whose real
	// target is itself local to the snapshot. sortedLocalFunctions excludes
	// the wrapper from the walk, so its own outgoing call is never
	// processed; returning a CallFact into it would dead-end the graph
	// silently, so this diagnostic makes that dead end visible instead. A
	// wrapper whose real target lies outside the snapshot was never going
	// to be walked either way and is not classified here -- it still
	// contributes an ordinary CallFact. A generic instantiation (fn.Pkg ==
	// nil, fn.Origin() != nil) is not classified here either, even when its
	// target is local: sortedLocalFunctions walks fn.Origin() directly (it
	// has its own Pkg and Blocks), so classifyCallSite routes the CallFact
	// to fn.Origin() instead of losing the edge.
	DiagCallUnresolvedSyntheticWrapper = "project_call_unresolved_synthetic_wrapper"
	// DiagCallGraphBudgetExceeded marks a call-graph build that stopped
	// before completion because a budget or context deadline was reached.
	DiagCallGraphBudgetExceeded = "project_callgraph_budget_exceeded"
	// DiagCallGraphBuildFailed marks a root whose Go sources could not be
	// loaded/type-checked, so no call-graph facts could be produced for it.
	DiagCallGraphBuildFailed = "project_callgraph_build_failed"
)

// CallGraphOptions bounds one BuildGoCallGraph call. Roots optionally scopes
// the build to specific repository-relative module roots, mirroring
// GoBuildOptions.Roots; when empty, BuildGoCallGraph builds every module
// root discovered under snapshot. Budgets.WallTime, MaxGraphNodes, and
// MaxGraphEdges are enforced here (see GoBudgets); MaxWorkingSetBytes
// remains reserved.
type CallGraphOptions struct {
	Roots   []string
	Budgets GoBudgets
}

// CallGraphResult is the bounded, versioned set of possible-call-reachability
// facts BuildGoCallGraph produces for a Snapshot. It is intentionally
// separate from Model: this is raw call-reachability evidence, not a
// dataflow proof or a Signal, and later work (source/sink registry, path
// traversal) is layered on top of it, not inside it.
type CallGraphResult struct {
	// CallFacts holds one entry per direct, statically resolved call edge
	// from a function in one of the snapshot's own packages. From/To are
	// ssa.Function.RelString(nil) identities ("pkgpath.Func" or
	// "(*pkgpath.Type).Method"): stable, deterministic, and independent of
	// the caller's absolute filesystem layout. Sorted by From then To.
	CallFacts []CallFact
	// Algorithm is CallGraphAlgorithm, echoed onto the result so callers do
	// not need to import projectmodel's const block to read it back.
	Algorithm string
	Coverage  Coverage
}

// frameworkRegistrationCallees are well-known HTTP registration functions
// whose arguments include a handler value invoked later by the framework,
// at a call site static analysis cannot see.
var frameworkRegistrationCallees = map[string]bool{
	"net/http.Handle":                 true,
	"net/http.HandleFunc":             true,
	"(*net/http.ServeMux).Handle":     true,
	"(*net/http.ServeMux).HandleFunc": true,
}

// BuildGoCallGraph builds bounded, deterministic Go call-graph facts for
// every module root under snapshot (or opts.Roots, if set) -- and only
// snapshot. It loads and type-checks the snapshot's Go sources via
// golang.org/x/tools/go/packages, which invokes the local Go toolchain
// against the materialized snapshot; that is expected (unlike root
// discovery, this build has no toolchain-avoidance constraint).
//
// Every ssa.CallInstruction reachable from the snapshot's own packages is
// classified: a direct, statically resolved call contributes a CallFact;
// interface dispatch, an unresolved function-value call, a
// reflect.Value.Call/CallSlice dispatch, a function/handler value passed to
// a recognized HTTP registration call, and a call into a synthetic
// bound-method-value or promoted-method thunk whose real target is itself
// local to the snapshot each contribute no CallFact but do contribute an
// explicit coverage diagnostic/count, so unresolved call sites stay visible
// rather than silently dropped; the synthetic-wrapper case is also the only
// one of these that marks Coverage.Complete false, since it means a real
// local-to-local call edge was structurally present but never walked.
//
// BuildGoCallGraph never returns a non-nil error for a per-root load
// failure or for budget/context exhaustion -- those are reported through
// Coverage.Diagnostics/Coverage.Complete instead, matching BuildGoModel's
// fail-open-with-diagnostics contract. It returns a non-nil error only if
// the snapshot itself cannot be materialized to build against.
func BuildGoCallGraph(ctx context.Context, snapshot fs.FS, opts CallGraphOptions) (CallGraphResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if opts.Budgets.WallTime > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Budgets.WallTime)
		defer cancel()
	}

	loaded, err := loadGoSnapshot(ctx, snapshot, opts.Roots, opts.Budgets)
	if err != nil {
		return CallGraphResult{}, err
	}
	defer loaded.cleanup()
	return buildGoCallGraphFromLoaded(ctx, loaded, opts), nil
}

func buildGoCallGraphFromLoaded(ctx context.Context, loaded *loadedGoSnapshot, opts CallGraphOptions) CallGraphResult {
	walk := newCallGraphWalk(loaded)

	buildResult := func(facts []CallFact, complete bool) CallGraphResult {
		return CallGraphResult{
			CallFacts: canonicalCallFacts(facts),
			Algorithm: CallGraphAlgorithm,
			Coverage: canonicalCoverage(Coverage{
				Phase:       "go_call_graph",
				Complete:    complete,
				Counts:      walk.counts,
				Budgets:     EffectiveGoBudgets(opts.Budgets),
				Diagnostics: walk.diagnostics,
			}),
		}
	}

	if ctx.Err() != nil && len(loaded.roots) == 0 {
		walk.diagnostics = append(walk.diagnostics, Diagnostic{Code: DiagCallGraphBudgetExceeded, Message: ctx.Err().Error()})
		return buildResult(nil, false)
	}

	walk.walkRoots(ctx, loaded, opts)

	if loaded.loadStopped && !containsDiagnosticCode(walk.diagnostics, DiagCallGraphBudgetExceeded) {
		path := ""
		if len(loaded.roots) < len(loaded.moduleDirs) {
			path = loaded.moduleDirs[len(loaded.roots)]
		}
		walk.diagnostics = append(walk.diagnostics, Diagnostic{Code: DiagCallGraphBudgetExceeded, Path: path})
	}

	if walk.truncated {
		walk.complete = false
	}
	return buildResult(walk.facts, walk.complete)
}

// sortedLocalFunctions returns every function with a body (fn.Blocks != nil)
// belonging to a package in localPkgPaths, sorted by RelString(nil) so
// budget truncation cuts the same trailing set on every call regardless of
// ssa/go-types' internal map iteration order.
func sortedLocalFunctions(prog *ssa.Program, localPkgPaths map[string]bool) []*ssa.Function {
	all := ssautil.AllFunctions(prog)
	out := make([]*ssa.Function, 0, len(all))
	for fn := range all {
		if fn.Blocks == nil || fn.Pkg == nil {
			continue
		}
		if !localPkgPaths[fn.Pkg.Pkg.Path()] {
			continue
		}
		out = append(out, fn)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RelString(nil) < out[j].RelString(nil) })
	return out
}

// stripTempDir removes materializeSnapshot's absolute temp-dir prefix from
// msg (as embedded by go/packages error text), so
// CallGraphResult.Coverage.Diagnostics stays deterministic across runs and
// across different absolute snapshot roots -- mirroring relCallSitePath's
// tempDir stripping for call-site paths.
func stripTempDir(msg, tempDir string) string {
	return strings.ReplaceAll(msg, tempDir, "")
}

func relCallSitePath(tempDir string, pos token.Position) string {
	if pos.Filename == "" {
		return ""
	}
	rel, err := filepath.Rel(tempDir, pos.Filename)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%s:%d", filepath.ToSlash(rel), pos.Line)
}

// materializeSnapshot copies snapshot into a new temporary directory so
// golang.org/x/tools/go/packages (which shells out to the Go toolchain) has
// real files to load; the caller must invoke the returned cleanup func.
func materializeSnapshot(snapshot fs.FS) (string, func(), error) {
	dir, err := os.MkdirTemp("", "projectmodel-callgraph-*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	if err := os.CopyFS(dir, snapshot); err != nil {
		cleanup()
		// dir is still returned (already removed by cleanup above) so a
		// caller can strip it from err's embedded absolute path via
		// stripTempDir before surfacing err in a diagnostic.
		return dir, func() {}, err
	}
	return dir, cleanup, nil
}
