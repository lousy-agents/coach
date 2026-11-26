package evaldetect

import (
	"fmt"
	"go/ast"
	"go/types"
	"sort"
)

// callSite is one statically resolved call, at its file, line and column.
type callSite struct {
	callee string
	file   string
	line   int
	column int
}

// funcNode is one declared function or method.
type funcNode struct {
	name   string
	dir    string
	source bool
	calls  []callSite
}

// callGraph is every local function, keyed by its full name, and the count
// of calls no static resolution reaches: through an interface or a function
// value.
type callGraph struct {
	nodes      map[string]*funcNode
	unresolved int
}

func buildCallGraph(program *goProgram) (callGraph, error) {
	graph := callGraph{nodes: map[string]*funcNode{}}
	for _, importPath := range program.importPaths() {
		pkg := program.packages[importPath]
		for i, file := range pkg.files {
			graph.addFile(program, pkg, pkg.fileNames[i], file)
		}
	}
	if len(graph.nodes) > maxCallGraphNodes {
		return callGraph{}, fmt.Errorf("%w: the call graph has more than %d functions (%d)", ErrRefused, maxCallGraphNodes, len(graph.nodes))
	}
	for _, node := range graph.nodes {
		sort.Slice(node.calls, func(i, j int) bool { return node.calls[i].before(node.calls[j]) })
	}
	return graph, nil
}

func (site callSite) before(other callSite) bool {
	if site.file != other.file {
		return site.file < other.file
	}
	if site.line != other.line {
		return site.line < other.line
	}
	return site.column < other.column
}

func (graph *callGraph) addFile(program *goProgram, pkg *goPackage, rel string, file *ast.File) {
	for _, decl := range file.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		object, ok := pkg.info.Defs[function.Name].(*types.Func)
		if !ok {
			continue
		}
		node := &funcNode{name: object.FullName(), dir: pkg.dir, source: isSourceShaped(object)}
		graph.nodes[node.name] = node
		ast.Inspect(function.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				graph.addCall(program, pkg, rel, node, call)
			}
			return true
		})
	}
}

// isSourceShaped reports whether a function can start a bypass: exported,
// and for a method, on an exported type.
func isSourceShaped(function *types.Func) bool {
	if !function.Exported() {
		return false
	}
	receiver := function.Type().(*types.Signature).Recv()
	if receiver == nil {
		return true
	}
	named, ok := derefNamed(receiver.Type())
	return ok && named.Obj().Exported()
}

func derefNamed(t types.Type) (*types.Named, bool) {
	if pointer, ok := t.(*types.Pointer); ok {
		t = pointer.Elem()
	}
	named, ok := t.(*types.Named)
	return named, ok
}
