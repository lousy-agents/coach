package evaldetect

import "sort"

// searchBypasses walks out from every source in an upstream layer, never
// entering a function in the required layer, and records each first
// function reached in a downstream layer. Breadth-first order over call
// sites sorted by file, line and column makes each witness the shortest
// path, with ties going to the earlier call.
func searchBypasses(graph callGraph, policy Policy) []Bypass {
	upstream, downstream := policy.bypassLayers()
	layerOf := func(node *funcNode) string { name, _ := policy.layerOf(node.dir); return name }
	var bypasses []Bypass
	for _, name := range sortedNames(graph.nodes) {
		source := graph.nodes[name]
		if source.source && upstream[layerOf(source)] {
			bypasses = append(bypasses, graph.bypassesFrom(source, policy, downstream, layerOf)...)
		}
	}
	return bypasses
}

func sortedNames(nodes map[string]*funcNode) []string {
	names := make([]string, 0, len(nodes))
	for name := range nodes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

type searchStep struct {
	node  *funcNode
	path  []*funcNode
	first callSite
}

func (graph callGraph) bypassesFrom(source *funcNode, policy Policy, downstream map[string]bool, layerOf func(*funcNode) string) []Bypass {
	search := bypassSearch{graph: graph, policy: policy, downstream: downstream, layerOf: layerOf, seen: map[string]bool{source.name: true}}
	queue := []searchStep{{node: source, path: []*funcNode{source}}}
	for len(queue) > 0 {
		step := queue[0]
		queue = queue[1:]
		for _, call := range step.node.calls {
			queue = append(queue, search.follow(step, call)...)
		}
	}
	return search.found
}

// bypassSearch is one source's breadth-first walk.
type bypassSearch struct {
	graph      callGraph
	policy     Policy
	downstream map[string]bool
	layerOf    func(*funcNode) string
	seen       map[string]bool
	found      []Bypass
}

// follow takes one call from step. It records a bypass when the call enters
// the downstream layer, and otherwise returns the step to explore next.
func (search *bypassSearch) follow(step searchStep, call callSite) []searchStep {
	next := search.graph.nodes[call.callee]
	if search.seen[call.callee] || search.layerOf(next) == search.policy.requiredLayer {
		return nil
	}
	search.seen[call.callee] = true
	advanced := searchStep{node: next, path: append(append([]*funcNode{}, step.path...), next), first: step.first}
	if len(step.path) == 1 {
		advanced.first = call
	}
	if search.downstream[search.layerOf(next)] {
		search.found = append(search.found, newBypass(advanced, search.policy))
		return nil
	}
	return []searchStep{advanced}
}

func newBypass(step searchStep, policy Policy) Bypass {
	source, sink := step.path[0].name, step.node.name
	bypass := Bypass{
		Finding: Finding{RuleID: layerBypass, Path: step.first.file, Subject: layerBypass + ":" + source + "->" + sink, Line: step.first.line},
		Source:  source,
		Sink:    sink,
	}
	for i, node := range step.path {
		bypass.Witness = append(bypass.Witness, node.name)
		if i > 0 && policy.forbids(step.path[i-1].dir, node.dir) {
			bypass.ViaForbiddenImport = true
		}
	}
	return bypass
}
