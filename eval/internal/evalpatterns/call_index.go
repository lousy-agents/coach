package evalpatterns

import (
	"go/ast"
	"strconv"
	"strings"
)

// declared is a top-level function and the file it is declared in, whose
// imports resolve its calls.
type declared struct {
	function *ast.FuncDecl
	file     *ast.File
	dir      string
}

// callIndex is every top-level function with a body, by package directory
// and name. Methods are not in it: syntax alone cannot say which method a
// call reaches.
type callIndex struct {
	module    string
	functions map[string]map[string]declared
}

func newCallIndex(module string, packages map[string][]*ast.File) callIndex {
	index := callIndex{module: module, functions: map[string]map[string]declared{}}
	for dir, files := range packages {
		index.functions[dir] = map[string]declared{}
		for _, file := range files {
			index.addFile(dir, file)
		}
	}
	return index
}

func (index callIndex) addFile(dir string, file *ast.File) {
	for _, function := range functionsWithBodies(file) {
		if function.Recv == nil {
			index.functions[dir][function.Name.Name] = declared{function, file, dir}
		}
	}
}

// resolve finds the function a call names by syntax: an identifier in the
// caller's own package, or pkg.F for a package of this module.
func (index callIndex) resolve(caller declared, fun ast.Expr) (declared, bool) {
	switch expr := fun.(type) {
	case *ast.Ident:
		callee, ok := index.functions[caller.dir][expr.Name]
		return callee, ok
	case *ast.SelectorExpr:
		pkg, isIdent := expr.X.(*ast.Ident)
		dir, local := index.localDir(caller.file, pkg, isIdent)
		callee, ok := index.functions[dir][expr.Sel.Name]
		return callee, local && ok
	}
	return declared{}, false
}

func (index callIndex) localDir(file *ast.File, pkg *ast.Ident, isIdent bool) (string, bool) {
	if !isIdent || index.module == "" {
		return "", false
	}
	for _, spec := range file.Imports {
		value, _ := strconv.Unquote(spec.Path.Value)
		if name, _ := nameOfImport(spec, value); name == pkg.Name {
			return moduleDir(index.module, value)
		}
	}
	return "", false
}

func moduleDir(module, importPath string) (string, bool) {
	if importPath == module {
		return ".", true
	}
	rest, ok := strings.CutPrefix(importPath, module+"/")
	return rest, ok
}
