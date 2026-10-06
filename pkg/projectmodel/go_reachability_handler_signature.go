package projectmodel

import (
	"go/types"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// httpHandlerFuncSignature looks up net/http.HandlerFunc's underlying
// *types.Signature from prog or the initial packages' type-checker import
// graph, returning nil if net/http was not part of this root's build.
func httpHandlerFuncSignature(prog *ssa.Program, pkgs []*packages.Package) *types.Signature {
	tp := typesPackageByPath(prog, pkgs, "net/http")
	if tp == nil {
		return nil
	}
	obj := tp.Scope().Lookup("HandlerFunc")
	if obj == nil {
		return nil
	}
	sig, ok := obj.Type().Underlying().(*types.Signature)
	if !ok {
		return nil
	}
	return sig
}
