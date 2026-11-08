package gitrepo

import (
	"go/ast"
)

// The calls in a file of a function with a given name.

// callsOf is the calls in file of a function with one of the names, as a function or as a member of a package.
func callsOf(file *ast.File, names ...string) []*ast.CallExpr {
	var calls []*ast.CallExpr
	ast.Inspect(file, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if isCall && calledByName(call, names) {
			calls = append(calls, call)
		}
		return true
	})
	return calls
}

func calledByName(call *ast.CallExpr, names []string) bool {
	var name string
	switch function := call.Fun.(type) {
	case *ast.Ident:
		name = function.Name
	case *ast.SelectorExpr:
		name = function.Sel.Name
	}
	for _, candidate := range names {
		if name == candidate {
			return true
		}
	}
	return false
}
