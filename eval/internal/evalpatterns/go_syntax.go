package evalpatterns

import (
	"go/ast"
	pathpkg "path"
	"strconv"
)

// paramTypes is the type of each parameter, one per name, so that
// (a, b T) gives T twice.
func paramTypes(params *ast.FieldList) []ast.Expr {
	var types []ast.Expr
	for _, field := range params.List {
		for range max(1, len(field.Names)) {
			types = append(types, field.Type)
		}
	}
	return types
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

