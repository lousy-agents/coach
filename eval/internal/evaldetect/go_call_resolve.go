package evaldetect

import (
	"go/ast"
	"go/types"
	"path"
)

// callOutcome is what a call resolves to.
type callOutcome int

const (
	callIgnored callOutcome = iota
	callResolved
	callUnresolved
)

// resolveCall resolves call statically to a local function or concrete
// method. A call through an interface or a function value is unresolved; a
// builtin, a conversion or a call into another module is ignored.
func resolveCall(program goProgram, pkg *goPackage, rel string, call *ast.CallExpr) (callSite, callOutcome) {
	callee, resolved := calledObject(pkg.info, call.Fun)
	if !resolved {
		return callSite{}, callUnresolved
	}
	function, ok := callee.(*types.Func)
	if !ok {
		return callSite{}, callIgnored
	}
	if receiver := function.Type().(*types.Signature).Recv(); receiver != nil && types.IsInterface(receiver.Type()) {
		return callSite{}, callUnresolved
	}
	if function.Pkg() == nil || program.packages[function.Pkg().Path()] == nil {
		return callSite{}, callIgnored
	}
	position := program.fset.Position(call.Pos())
	return callSite{callee: function.FullName(), file: path.Clean(rel), line: position.Line, column: position.Column}, callResolved
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
