package semantics

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// computeReactComponents discovers every candidate React component among
// root's top-level exports (TS/TSX only) and extracts its useState
// bindings plus its coordination facts (CoordinatedTransitions,
// WorkspaceBranches, ImperativeUI, SharedPanelDeps).
func computeReactComponents(root engine.Node, source []byte) []ReactComponentFacts {
	if root == nil {
		return nil
	}

	hasDirective := moduleHasUseClientDirective(root, source)
	bindings := collectModuleTopLevelBindings(root, source)
	out := reactCollectExportedComponents(root, source, hasDirective, bindings)

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Location.StartByte != out[j].Location.StartByte {
			return out[i].Location.StartByte < out[j].Location.StartByte
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func reactCollectExportedComponents(root engine.Node, source []byte, hasDirective bool, bindings map[string]engine.Node) []ReactComponentFacts {
	var out []ReactComponentFacts
	for _, exportStmt := range reactExportStatements(root) {
		out = append(out, reactFactsFromExport(exportStmt, source, hasDirective, bindings)...)
	}
	return reactDedupeComponentsBySpan(out)
}

func reactExportStatements(root engine.Node) []engine.Node {
	var out []engine.Node
	count := root.ChildCount()
	for i := 0; i < count; i++ {
		child := root.Child(i)
		if child.Kind() == "export_statement" {
			out = append(out, child)
		}
	}
	return out
}

func reactFactsFromExport(exportStmt engine.Node, source []byte, hasDirective bool, bindings map[string]engine.Node) []ReactComponentFacts {
	var out []ReactComponentFacts
	for _, cand := range reactExportedCandidates(exportStmt, source, bindings) {
		rec, ok := reactBuildComponentFacts(cand, hasDirective, source)
		if !ok {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func reactDedupeComponentsBySpan(in []ReactComponentFacts) []ReactComponentFacts {
	seen := map[[2]uint]struct{}{}
	var out []ReactComponentFacts
	for _, rec := range in {
		span := [2]uint{rec.Location.StartByte, rec.Location.EndByte}
		if _, dup := seen[span]; dup {
			continue
		}
		seen[span] = struct{}{}
		out = append(out, rec)
	}
	return out
}
