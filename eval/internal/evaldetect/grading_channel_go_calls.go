package evaldetect

import (
	"go/ast"
	"strconv"
	"strings"
)

func isSelector(selector *ast.SelectorExpr, pkg, name string) bool {
	ident, ok := selector.X.(*ast.Ident)
	return ok && ident.Name == pkg && (name == "" || selector.Sel.Name == name)
}

// callsOnInheritedFD reports a call into syscall or x/sys/unix whose first argument is a descriptor literal of 3 or more.
func callsOnInheritedFD(call *ast.CallExpr, names goImportNames) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) == 0 || !(isSelector(selector, names.syscall, "") || isSelector(selector, names.unix, "")) {
		return false
	}
	literal, ok := call.Args[0].(*ast.BasicLit)
	if !ok {
		return false
	}
	number, err := strconv.ParseInt(literal.Value, 0, 64)
	return err == nil && number >= firstInheritedFD
}

// importedAs is the name the file gives the package at path, or empty when it does not import it.
func importedAs(file *ast.File, path string) string {
	for _, spec := range file.Imports {
		if unquoted, err := strconv.Unquote(spec.Path.Value); err != nil || unquoted != path {
			continue
		}
		if spec.Name != nil {
			return spec.Name.Name
		}
		return path[strings.LastIndex(path, "/")+1:]
	}
	return ""
}
