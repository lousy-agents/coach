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

func buildCallGraph(program goProgram) (callGraph, error) {
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

func (graph *callGraph) addFile(program goProgram, pkg *goPackage, rel string, file *ast.File) {
	for _, decl := range file.Decls {
		if function, ok := decl.(*ast.FuncDecl); ok && function.Body != nil {
			graph.addFunction(program, pkg, rel, function)
		}
	}
}

func (graph *callGraph) addFunction(program goProgram, pkg *goPackage, rel string, function *ast.FuncDecl) {
	object, ok := pkg.info.Defs[function.Name].(*types.Func)
	if !ok {
		return
	}
	node := &funcNode{name: object.FullName(), dir: pkg.dir, source: isSourceShaped(object)}
	graph.nodes[node.name] = node
	ast.Inspect(function.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		site, outcome := resolveCall(program, pkg, rel, call)
		switch outcome {
		case callResolved:
			node.calls = append(node.calls, site)
		case callUnresolved:
			graph.unresolved++
		}
		return true
	})
}
