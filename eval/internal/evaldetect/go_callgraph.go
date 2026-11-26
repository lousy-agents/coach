package evaldetect

import (
	"fmt"
	"go/ast"
	"go/types"
)

// callSite is one statically resolved call, at its file, line and column.
type callSite struct {
	callee string
	file   string
	line   int
	column int
}

// funcNode is one declared function or method, with its calls in source
// order: ast.Inspect visits a body by position. A function declared without
// a body has a node with no calls, so it can end a path as a sink.
type funcNode struct {
	name     string
	dir      string
	source   bool
	bodyless bool
	calls    []callSite
}

// callGraph is every local function, keyed by its full name, and the count
// of calls the search cannot follow: through an interface or a function
// value, or to a function no node stands for. A call to a bodyless function
// is counted by the search instead, which knows whether it is a sink.
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
	graph.dropCallsOutsideTheGraph()
	return graph, nil
}

// dropCallsOutsideTheGraph counts a call to a function that has no node,
// such as one declared without a body, as unresolved, so the search only
// follows calls it can continue from.
func (graph *callGraph) dropCallsOutsideTheGraph() {
	for _, node := range graph.nodes {
		var dropped int
		node.calls, dropped = graph.callsInTheGraph(node.calls)
		graph.unresolved += dropped
	}
}

func (graph *callGraph) callsInTheGraph(calls []callSite) ([]callSite, int) {
	kept := calls[:0]
	for _, call := range calls {
		if graph.nodes[call.callee] != nil {
			kept = append(kept, call)
		}
	}
	return kept, len(calls) - len(kept)
}

func (graph *callGraph) addFile(program goProgram, pkg *goPackage, rel string, file *ast.File) {
	for _, decl := range file.Decls {
		if function, ok := decl.(*ast.FuncDecl); ok {
			graph.addFunction(program, pkg, rel, function)
		}
	}
}

func (graph *callGraph) addFunction(program goProgram, pkg *goPackage, rel string, function *ast.FuncDecl) {
	object, ok := pkg.info.Defs[function.Name].(*types.Func)
	if !ok {
		return
	}
	node := &funcNode{name: object.FullName(), dir: pkg.dir, source: isSourceShaped(object), bodyless: function.Body == nil}
	graph.nodes[node.name] = node
	if node.bodyless {
		return
	}
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
