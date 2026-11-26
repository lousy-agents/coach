package evalpatterns

import (
	"go/ast"
	"strings"
	"unicode"
)

// allocatingBuiltins are calls that build a value without reaching outside
// the constructor, so they do not make it do work.
var allocatingBuiltins = map[string]bool{"append": true, "cap": true, "len": true, "make": true, "new": true}

// countConstructor classifies a New… function: assigns_only when its body
// is only assignments and returns with no call but an allocating builtin,
// does_work otherwise (Hevery's "constructor does real work").
func countConstructor(shapes *constructorShapes, function *ast.FuncDecl) {
	if !isConstructorName(function) {
		return
	}
	if onlyAssigns(function.Body) {
		shapes.AssignsOnly++
		return
	}
	shapes.DoesWork++
}

func isConstructorName(function *ast.FuncDecl) bool {
	rest, ok := strings.CutPrefix(function.Name.Name, "New")
	return ok && function.Recv == nil && function.Type.Results != nil &&
		(rest == "" || unicode.IsUpper([]rune(rest)[0]))
}

func onlyAssigns(body *ast.BlockStmt) bool {
	for _, statement := range body.List {
		switch statement.(type) {
		case *ast.AssignStmt, *ast.DeclStmt, *ast.ReturnStmt:
		default:
			return false
		}
	}
	return !callsOutward(body)
}

func callsOutward(body *ast.BlockStmt) bool {
	outward := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return !outward
		}
		if ident, isIdent := call.Fun.(*ast.Ident); !isIdent || !allocatingBuiltins[ident.Name] {
			outward = true
		}
		return !outward
	})
	return outward
}
