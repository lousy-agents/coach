package projectmodel

import (
	"go/types"

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

func frameworkRegistrationDiagnostics(callee *ssa.Function, common *ssa.CallCommon, sitePath string, httpHandlerIface *types.Interface) []Diagnostic {
	calleeID := callee.RelString(nil)
	if !frameworkRegistrationCallees[calleeID] {
		return nil
	}
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
	var diags []Diagnostic
	for _, arg := range args {
		if isFunctionValueArg(arg, httpHandlerIface) {
			diags = append(diags, Diagnostic{Code: DiagCallUnresolvedFrameworkRegistration, Path: sitePath})
		}
	}
	return diags
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
