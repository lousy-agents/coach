package projectmodel

import (
	"go/types"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// frameworkRegistrationCallees are well-known HTTP registration functions
// whose arguments include a handler value invoked later by the framework,
// at a call site static analysis cannot see.
var frameworkRegistrationCallees = map[string]bool{
	"net/http.Handle":                 true,
	"net/http.HandleFunc":             true,
	"(*net/http.ServeMux).Handle":     true,
	"(*net/http.ServeMux).HandleFunc": true,
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
