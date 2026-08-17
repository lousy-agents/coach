package projectmodel

import (
	"context"
	"fmt"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/callgraph/static"
	"golang.org/x/tools/go/packages"
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
// reflect.Value.Call/CallSlice dispatch, and a function/handler value
// passed to a recognized HTTP registration call each contribute no
// CallFact but do contribute an explicit coverage diagnostic/count, so
// unresolved call sites stay visible rather than silently dropped.
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
	diagnostics := append([]Diagnostic{}, loaded.discovery.Diagnostics...)
	complete := loaded.discovery.Complete

	counts := map[string]int{
		"roots_seen":                        len(loaded.moduleDirs),
		"roots_built":                       0,
		"functions_seen":                    0,
		"call_sites_seen":                   0,
		"unresolved_interface":              0,
		"unresolved_function_value":         0,
		"unresolved_reflection":             0,
		"unresolved_framework_registration": 0,
		"callgraph_static_nodes":            0,
		"ssa_programs_built":                loaded.programsBuilt(),
	}

	buildResult := func(facts []CallFact, complete bool) CallGraphResult {
		return CallGraphResult{
			CallFacts: canonicalCallFacts(facts),
			Algorithm: CallGraphAlgorithm,
			Coverage: canonicalCoverage(Coverage{
				Phase:       "go_call_graph",
				Complete:    complete,
				Counts:      counts,
				Budgets:     EffectiveGoBudgets(opts.Budgets),
				Diagnostics: diagnostics,
			}),
		}
	}

	if ctx.Err() != nil && len(loaded.roots) == 0 {
		diagnostics = append(diagnostics, Diagnostic{Code: DiagCallGraphBudgetExceeded, Message: ctx.Err().Error()})
		return buildResult(nil, false)
	}

	var callFacts []CallFact
	nodesProcessed, edgesProcessed := 0, 0
	truncated := loaded.loadStopped

rootLoop:
	for _, root := range loaded.roots {
		if ctx.Err() != nil {
			truncated = true
			diagnostics = append(diagnostics, Diagnostic{Code: DiagCallGraphBudgetExceeded, Path: root.dir})
			break
		}

		if root.loadErr != nil {
			complete = false
			diagnostics = append(diagnostics, Diagnostic{Code: DiagCallGraphBuildFailed, Path: root.dir, Message: stripTempDir(root.loadErr.Error(), loaded.tempDir)})
			continue
		}
		for _, p := range root.pkgs {
			for _, e := range p.Errors {
				complete = false
				diagnostics = append(diagnostics, Diagnostic{Code: DiagCallGraphBuildFailed, Path: root.dir, Message: stripTempDir(e.Error(), loaded.tempDir)})
			}
		}
		counts["roots_built"]++

		// CallFacts deliberately come from the sortedLocalFunctions walk
		// below, not from cg.Nodes/cg.Edges: that walk gives a single,
		// deterministically ordered pass that both resolves direct callees
		// and classifies every unresolved call site (interface, function
		// value, reflection, framework registration) in one place. Walking
		// cg's own maps would reintroduce Go map iteration order and drop
		// the unresolved-site diagnostics; cg is kept only for the pinned
		// static-analysis-backend cross-check below.
		cg := static.CallGraph(root.prog)
		counts["callgraph_static_nodes"] += len(cg.Nodes)
		httpHandlerIface := httpHandlerInterface(root.prog, root.pkgs)

		for _, fn := range sortedLocalFunctions(root.prog, root.localPkgPaths) {
			if ctx.Err() != nil {
				truncated = true
				diagnostics = append(diagnostics, Diagnostic{Code: DiagCallGraphBudgetExceeded, Path: root.dir})
				break rootLoop
			}
			if opts.Budgets.MaxGraphNodes > 0 && nodesProcessed >= opts.Budgets.MaxGraphNodes {
				truncated = true
				diagnostics = append(diagnostics, Diagnostic{Code: DiagCallGraphBudgetExceeded, Path: root.dir})
				break rootLoop
			}
			nodesProcessed++
			counts["functions_seen"]++

			for _, blk := range fn.Blocks {
				for _, instr := range blk.Instrs {
					site, ok := instr.(ssa.CallInstruction)
					if !ok {
						continue
					}
					if opts.Budgets.MaxGraphEdges > 0 && edgesProcessed >= opts.Budgets.MaxGraphEdges {
						truncated = true
						diagnostics = append(diagnostics, Diagnostic{Code: DiagCallGraphBudgetExceeded, Path: root.dir})
						break rootLoop
					}
					edgesProcessed++
					counts["call_sites_seen"]++

					result := classifyCallSite(fn, site, loaded.tempDir, httpHandlerIface)
					if result.Fact != nil {
						callFacts = append(callFacts, *result.Fact)
					}
					for _, d := range result.Diagnostics {
						diagnostics = append(diagnostics, d)
						counts[callSiteDiagnosticCounts[d.Code]]++
					}
				}
			}
		}
	}

	if loaded.loadStopped && !containsDiagnosticCode(diagnostics, DiagCallGraphBudgetExceeded) {
		path := ""
		if len(loaded.roots) < len(loaded.moduleDirs) {
			path = loaded.moduleDirs[len(loaded.roots)]
		}
		diagnostics = append(diagnostics, Diagnostic{Code: DiagCallGraphBudgetExceeded, Path: path})
	}

	if truncated {
		complete = false
	}
	return buildResult(callFacts, complete)
}

// callSiteDiagnosticCounts maps each call-site diagnostic code classifyCallSite
// can emit to the Coverage.Counts key BuildGoCallGraph increments for it.
var callSiteDiagnosticCounts = map[string]string{
	DiagCallUnresolvedInterface:             "unresolved_interface",
	DiagCallUnresolvedFunctionValue:         "unresolved_function_value",
	DiagCallUnresolvedReflection:            "unresolved_reflection",
	DiagCallUnresolvedFrameworkRegistration: "unresolved_framework_registration",
}

// callSiteClassification is the result of classifying one ssa.CallInstruction:
// an optional resolved CallFact plus zero or more coverage diagnostics.
// Interface dispatch, an unresolved function value, and a reflection
// dispatch each contribute exactly one diagnostic and no CallFact; a
// resolved direct call to a frameworkRegistrationCallees entry contributes
// its CallFact plus zero or more diagnostics, one per handler-typed
// argument.
type callSiteClassification struct {
	Fact        *CallFact
	Diagnostics []Diagnostic
}

// classifyCallSite resolves site's callee within fn and reports the
// resulting CallFact and/or diagnostics. It touches no shared call-graph
// state -- BuildGoCallGraph merges the result into its own
// callFacts/counts/diagnostics.
func classifyCallSite(fn *ssa.Function, site ssa.CallInstruction, tempDir string, httpHandlerIface *types.Interface) callSiteClassification {
	sitePath := relCallSitePath(tempDir, fn.Prog.Fset.Position(site.Pos()))
	common := site.Common()

	if common.IsInvoke() {
		return callSiteClassification{Diagnostics: []Diagnostic{{Code: DiagCallUnresolvedInterface, Path: sitePath}}}
	}

	callee := common.StaticCallee()
	if callee == nil {
		return callSiteClassification{Diagnostics: []Diagnostic{{Code: DiagCallUnresolvedFunctionValue, Path: sitePath}}}
	}

	if isReflectDynamicCall(callee) {
		return callSiteClassification{Diagnostics: []Diagnostic{{Code: DiagCallUnresolvedReflection, Path: sitePath}}}
	}

	calleeID := callee.RelString(nil)
	result := callSiteClassification{Fact: &CallFact{From: fn.RelString(nil), To: calleeID}}

	if frameworkRegistrationCallees[calleeID] {
		args := common.Args
		if callee.Signature.Recv() != nil && len(args) > 0 {
			// Method-form registration ((*http.ServeMux).Handle/
			// HandleFunc): Args[0] is the receiver, not a handler
			// argument -- see ssa.CallCommon.Args's doc ("If Value
			// is a method, Args[0] contains the receiver
			// parameter"). *http.ServeMux itself implements
			// http.Handler, so skipping it here avoids
			// double-counting the registration site.
			args = args[1:]
		}
		for _, arg := range args {
			if isFunctionValueArg(arg, httpHandlerIface) {
				result.Diagnostics = append(result.Diagnostics, Diagnostic{Code: DiagCallUnresolvedFrameworkRegistration, Path: sitePath})
			}
		}
	}

	return result
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

// isReflectDynamicCall reports whether fn is reflect.Value.Call or
// reflect.Value.CallSlice: SSA resolves both as ordinary static method
// calls (reflect.Value is a concrete type), but the function they actually
// invoke is chosen at runtime and is invisible to static analysis.
func isReflectDynamicCall(fn *ssa.Function) bool {
	if fn.Pkg == nil || fn.Pkg.Pkg.Path() != "reflect" {
		return false
	}
	switch fn.Name() {
	case "Call", "CallSlice":
		return true
	default:
		return false
	}
}

// isFunctionValueArg reports whether v is a handler value passed to a
// frameworkRegistrationCallees entry: either a func-typed value (the
// net/http.HandleFunc/(*http.ServeMux).HandleFunc case) or a value whose
// type implements net/http.Handler (the net/http.Handle/
// (*http.ServeMux).Handle case, where the parameter type is the interface,
// not a func signature). handlerIface is nil when net/http was not loaded
// for this root, in which case only the func-typed check applies.
func isFunctionValueArg(v ssa.Value, handlerIface *types.Interface) bool {
	t := v.Type()
	if _, ok := t.Underlying().(*types.Signature); ok {
		return true
	}
	return handlerIface != nil && types.Implements(t, handlerIface)
}

// httpHandlerInterface looks up net/http.Handler's interface type from
// prog or the initial packages' type-checker import graph, returning nil
// if net/http was not part of this root's build (so isFunctionValueArg
// falls back to its func-typed check only).
func httpHandlerInterface(prog *ssa.Program, pkgs []*packages.Package) *types.Interface {
	tp := typesPackageByPath(prog, pkgs, "net/http")
	if tp == nil {
		return nil
	}
	obj := tp.Scope().Lookup("Handler")
	if obj == nil {
		return nil
	}
	iface, ok := obj.Type().Underlying().(*types.Interface)
	if !ok {
		return nil
	}
	return iface
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
