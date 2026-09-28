package semantics

import (
	"sort"

	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// computeGoCognitiveComplexity discovers every scored Go function body under
// root and returns one FunctionCognitiveComplexity record per body (including
// zero scores), ordered by ascending location.start_byte then name.
func computeGoCognitiveComplexity(root engine.Node, source []byte) []FunctionCognitiveComplexity {
	if root == nil {
		return nil
	}
	targets := collectGoCCTargets(root, source)
	if len(targets) == 0 {
		return nil
	}
	records := make([]FunctionCognitiveComplexity, 0, len(targets))
	for _, t := range targets {
		body := t.node.ChildByFieldName("body")
		score := 0
		if body != nil {
			s := &goCCScorer{source: source, funcName: t.name}
			s.walk(body, 0)
			score = s.score
		}
		records = append(records, FunctionCognitiveComplexity{
			Name:     t.name,
			Kind:     t.kind,
			Location: locationFromNode(t.node),
			Score:    score,
		})
	}
	sort.SliceStable(records, func(i, j int) bool {
		if records[i].Location.StartByte != records[j].Location.StartByte {
			return records[i].Location.StartByte < records[j].Location.StartByte
		}
		return records[i].Name < records[j].Name
	})
	return records
}
func singleIdentifierName(list engine.Node, source []byte) string {
	if list == nil {
		return ""
	}
	var id string
	count := list.ChildCount()
	for i := 0; i < count; i++ {
		child := list.Child(i)
		switch child.Kind() {
		case ",":
			continue
		case "identifier":
			if id != "" {
				return ""
			}
			id = child.Utf8Text(source)
		default:
			return ""
		}
	}
	return id
}
func isGoBooleanBinary(n engine.Node, source []byte) bool {
	if n == nil || n.Kind() != "binary_expression" {
		return false
	}
	op := goBinaryOp(n, source)
	return op == "&&" || op == "||"
}
