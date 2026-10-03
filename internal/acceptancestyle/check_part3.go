package acceptancestyle

import (
	"go/ast"
)

func callInvokesGinkgoSpecIdent(fun ast.Expr, importNames map[string]struct{}) bool {
	switch fun := fun.(type) {
	case *ast.Ident:
		if _, dot := importNames["."]; !dot {
			return false
		}
		_, ok := ginkgoSpecIdents[fun.Name]
		return ok
	case *ast.SelectorExpr:
		pkg, ok := fun.X.(*ast.Ident)
		if !ok {
			return false
		}
		if _, ok := importNames[pkg.Name]; !ok {
			return false
		}
		_, ok = ginkgoSpecIdents[fun.Sel.Name]
		return ok
	default:
		return false
	}
}
func referencesGinkgoSpec(f *ast.File, importNames map[string]struct{}) bool {
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if callInvokesGinkgoSpecIdent(call.Fun, importNames) {
			found = true
		}
		return true
	})
	return found
}
