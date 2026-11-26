package evalpatterns

import "go/ast"

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

// walk is one breadth-first search from an entry.
type walk struct {
	index callIndex
	seen  map[*ast.FuncDecl]bool
}

// depthFrom is the greatest breadth-first distance reached from start.
func (index callIndex) depthFrom(start declared) int {
	search := walk{index: index, seen: map[*ast.FuncDecl]bool{start.function: true}}
	frontier, depth := []declared{start}, 0
	for len(frontier) > 0 {
		frontier = search.step(frontier)
		if len(frontier) > 0 {
			depth++
		}
	}
	return depth
}

// step is every function the frontier calls that the walk has not reached.
func (search *walk) step(frontier []declared) []declared {
	var next []declared
	for _, caller := range frontier {
		next = append(next, search.unseenCallees(caller)...)
	}
	return next
}

func (search *walk) unseenCallees(caller declared) []declared {
	var callees []declared
	ast.Inspect(caller.function.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			callees = append(callees, search.reach(caller, call)...)
		}
		return true
	})
	return callees
}

func (search *walk) reach(caller declared, call *ast.CallExpr) []declared {
	callee, found := search.index.resolve(caller, call.Fun)
	if !found || search.seen[callee.function] {
		return nil
	}
	search.seen[callee.function] = true
	return []declared{callee}
}
