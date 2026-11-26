package evaldetect

import (
	"go/ast"
	"go/types"
	"path"
)

// addCall records call as an edge when it resolves statically to a local
// function or concrete method, counts it unresolved when it goes through an
// interface or a function value, and ignores builtins, conversions and calls
// into other modules.
func (graph *callGraph) addCall(program *goProgram, pkg *goPackage, rel string, caller *funcNode, call *ast.CallExpr) {
	callee, resolved := calledObject(pkg.info, call.Fun)
	if !resolved {
		graph.unresolved++
		return
	}
	function, ok := callee.(*types.Func)
	if !ok {
		return
	}
	if receiver := function.Type().(*types.Signature).Recv(); receiver != nil && types.IsInterface(receiver.Type()) {
		graph.unresolved++
		return
	}
	if function.Pkg() == nil || program.packages[function.Pkg().Path()] == nil {
		return
	}
	position := program.fset.Position(call.Pos())
	caller.calls = append(caller.calls, callSite{callee: function.FullName(), file: path.Clean(rel), line: position.Line, column: position.Column})
}

// calledObject names what a call's function expression refers to. It
// reports false for a call through a value: a variable, a field, a function
// literal or any other expression.
func calledObject(info *types.Info, fun ast.Expr) (types.Object, bool) {
	switch expr := ast.Unparen(fun).(type) {
	case *ast.Ident:
		return objectOrFalse(info.Uses[expr])
	case *ast.SelectorExpr:
		if selection, ok := info.Selections[expr]; ok {
			if selection.Kind() == types.FieldVal {
				return nil, false
			}
			return selection.Obj(), true
		}
		return objectOrFalse(info.Uses[expr.Sel])
	case *ast.IndexExpr, *ast.IndexListExpr, *ast.ArrayType, *ast.MapType, *ast.ChanType, *ast.FuncType, *ast.InterfaceType, *ast.StructType:
		return nil, true
	}
	return nil, false
}

func objectOrFalse(object types.Object) (types.Object, bool) {
	if _, isVar := object.(*types.Var); isVar || object == nil {
		return nil, false
	}
	return object, true
}
