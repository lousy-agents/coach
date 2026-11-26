package evalpatterns

import (
	"go/ast"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	pathpkg "path"
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
			for _, function := range functionsWithBodies(file) {
				if function.Recv == nil {
					index.functions[dir][function.Name.Name] = declared{function, file, dir}
				}
			}
		}
	}
	return index
}

// entryDepths is, for each entry in sorted package order, the longest
// shortest call path from it.
func entryDepths(module string, packages map[string][]*ast.File) []int {
	index := newCallIndex(module, packages)
	var depths []int
	for _, dir := range sortedDirs(packages) {
		for _, file := range packages[dir] {
			depths = append(depths, index.fileEntryDepths(file, dir)...)
		}
	}
	return depths
}

func (index callIndex) fileEntryDepths(file *ast.File, dir string) []int {
	var depths []int
	for _, function := range functionsWithBodies(file) {
		if entryKindOf(file, function) != "" {
			depths = append(depths, index.depthFrom(declared{function, file, dir}))
		}
	}
	return depths
}

// depthFrom is the greatest breadth-first distance reached from start.
func (index callIndex) depthFrom(start declared) int {
	seen := map[*ast.FuncDecl]bool{start.function: true}
	frontier, depth := []declared{start}, 0
	for len(frontier) > 0 {
		var next []declared
		for _, caller := range frontier {
			next = append(next, index.unseenCallees(caller, seen)...)
		}
		if len(next) > 0 {
			depth++
		}
		frontier = next
	}
	return depth
}

func (index callIndex) unseenCallees(caller declared, seen map[*ast.FuncDecl]bool) []declared {
	var callees []declared
	ast.Inspect(caller.function.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if callee, found := index.resolve(caller, call.Fun); found && !seen[callee.function] {
			seen[callee.function] = true
			callees = append(callees, callee)
		}
		return true
	})
	return callees
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

// modulePath is the module path in root's go.mod, or "" without one.
func modulePath(root string) string {
	raw, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(raw), "\n") {
		if rest, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return ""
}

func sortedDirs(packages map[string][]*ast.File) []string {
	dirs := make([]string, 0, len(packages))
	for dir := range packages {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)
	return dirs
}

func packageDir(rel string) string { return pathpkg.Dir(rel) }
