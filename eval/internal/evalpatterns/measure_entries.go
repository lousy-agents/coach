package evalpatterns

import (
	"go/ast"
	pathpkg "path"
	"strconv"
)

// countEntry classifies an exported function by its signature, read from
// syntax alone: an http.Handler-shaped function or ServeHTTP method, any
// other method on an exported type, or a plain function.
func countEntry(kinds *entryKinds, file *ast.File, function *ast.FuncDecl) {
	if !function.Name.IsExported() {
		return
	}
	switch {
	case isHTTPHandler(file, function):
		kinds.HTTPHandler++
	case function.Recv != nil:
		if exportedReceiver(function.Recv) {
			kinds.Method++
		}
	default:
		kinds.Function++
	}
}

func isHTTPHandler(file *ast.File, function *ast.FuncDecl) bool {
	http := importName(file, "net/http")
	if http == "" {
		return false
	}
	params := function.Type.Params.List
	var types []ast.Expr
	for _, field := range params {
		for range max(1, len(field.Names)) {
			types = append(types, field.Type)
		}
	}
	return len(types) == 2 && isSelector(types[0], http, "ResponseWriter") && isPointerTo(types[1], http, "Request") &&
		(function.Recv == nil || function.Name.Name == "ServeHTTP")
}

func isSelector(expr ast.Expr, pkg, name string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == pkg && selector.Sel.Name == name
}

func isPointerTo(expr ast.Expr, pkg, name string) bool {
	star, ok := expr.(*ast.StarExpr)
	return ok && isSelector(star.X, pkg, name)
}

// importName is the name file uses for the package at path, or "" when it
// does not import it.
func importName(file *ast.File, path string) string {
	for _, spec := range file.Imports {
		if value, _ := strconv.Unquote(spec.Path.Value); value == path {
			if spec.Name != nil {
				return spec.Name.Name
			}
			return pathpkg.Base(value)
		}
	}
	return ""
}

func exportedReceiver(receiver *ast.FieldList) bool {
	expr := receiver.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch base := expr.(type) {
	case *ast.IndexExpr:
		expr = base.X
	case *ast.IndexListExpr:
		expr = base.X
	}
	ident, ok := expr.(*ast.Ident)
	return ok && ident.IsExported()
}
