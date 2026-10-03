package projectmodel

import (
	"go/types"

	"golang.org/x/tools/go/ssa"
)

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
