package projectmodel

import (
	"fmt"
	"go/token"
	"go/types"
	"path/filepath"

	"golang.org/x/tools/go/ssa"
)

// callSiteDiagnosticCounts maps each call-site diagnostic code classifyCallSite
// can emit to the Coverage.Counts key BuildGoCallGraph increments for it.
var callSiteDiagnosticCounts = map[string]string{
	DiagCallUnresolvedInterface:             "unresolved_interface",
	DiagCallUnresolvedFunctionValue:         "unresolved_function_value",
	DiagCallUnresolvedReflection:            "unresolved_reflection",
	DiagCallUnresolvedFrameworkRegistration: "unresolved_framework_registration",
	DiagCallUnresolvedSyntheticWrapper:      "unresolved_synthetic_wrapper",
}

// callSiteClassification is the result of classifying one ssa.CallInstruction:
// an optional resolved CallFact plus zero or more coverage diagnostics.
// Interface dispatch, an unresolved function value, a reflection dispatch,
// and a call into a local-targeted synthetic bound-method-value or
// promoted-method thunk (not a generic instantiation -- see
// DiagCallUnresolvedSyntheticWrapper) each contribute exactly one
// diagnostic and no CallFact; a resolved direct call to a
// frameworkRegistrationCallees entry contributes its CallFact plus zero or
// more diagnostics, one per handler-typed argument.
type callSiteClassification struct {
	Fact        *CallFact
	Diagnostics []Diagnostic
}

// classifyCallSite resolves site's callee within fn and reports the
// resulting CallFact and/or diagnostics. It touches no shared call-graph
// state -- BuildGoCallGraph merges the result into its own
// callFacts/counts/diagnostics. localPkgPaths is root's local package set,
// used only to decide whether a synthetic wrapper's real target was ever
// reachable from sortedLocalFunctions.
func classifyCallSite(fn *ssa.Function, site ssa.CallInstruction, tempDir string, httpHandlerIface *types.Interface, localPkgPaths map[string]bool) callSiteClassification {
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

	callee, lost := rewriteLocalSyntheticWrapper(callee, localPkgPaths)
	if lost {
		return callSiteClassification{Diagnostics: []Diagnostic{{Code: DiagCallUnresolvedSyntheticWrapper, Path: sitePath}}}
	}

	return callSiteClassification{
		Fact:        &CallFact{From: fn.RelString(nil), To: callee.RelString(nil)},
		Diagnostics: frameworkRegistrationDiagnostics(callee, common, sitePath, httpHandlerIface),
	}
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

// rewriteLocalSyntheticWrapper rewrites a call into a synthetic wrapper
// (fn.Pkg == nil) whose real target is local to the snapshot. Generic
// instantiations (Origin() != nil) route to the origin function
// sortedLocalFunctions already walks; bound-method-value and
// promoted-method wrappers have no such origin and report lost=true so
// the caller can emit DiagCallUnresolvedSyntheticWrapper instead of a
// dead-end CallFact. External-target wrappers and non-wrappers are
// returned unchanged with lost=false.
func rewriteLocalSyntheticWrapper(callee *ssa.Function, localPkgPaths map[string]bool) (*ssa.Function, bool) {
	if callee.Pkg != nil || !localPkgPaths[syntheticWrapperTargetPkgPath(callee)] {
		return callee, false
	}
	if origin := callee.Origin(); origin != nil {
		return origin, false
	}
	return callee, true
}

// syntheticWrapperTargetPkgPath returns the package path of the function or
// method a synthetic wrapper (fn.Pkg == nil) actually delegates to, or ""
// if fn is not such a wrapper. Bound-method-value wrappers, promoted/
// embedded-method thunks, and generic instantiations all set fn.Object() to
// the *types.Func being wrapped/instantiated, even though fn.Pkg itself is
// nil; go/ssa's wrappers.go/instantiate.go set this field, not any public
// API, so this is the only way to recover the real target's package.
func syntheticWrapperTargetPkgPath(fn *ssa.Function) string {
	obj := fn.Object()
	if obj == nil || obj.Pkg() == nil {
		return ""
	}
	return obj.Pkg().Path()
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

// unresolvedCallSiteCount sums the five classifyCallSite unresolved-site
// Coverage.Counts keys so reachability and layer-bypass stay in lockstep
// when a new class is added.
func unresolvedCallSiteCount(counts map[string]int) int {
	return counts["unresolved_interface"] +
		counts["unresolved_function_value"] +
		counts["unresolved_reflection"] +
		counts["unresolved_framework_registration"] +
		counts["unresolved_synthetic_wrapper"]
}
