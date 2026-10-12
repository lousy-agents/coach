package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// collectSyntaxIssues walks the tree rooted at n in pre-order and collects a
// SyntaxIssue for every ERROR or MISSING node found.
func collectSyntaxIssues(n engine.Node) []SyntaxIssue {
	var issues []SyntaxIssue
	var walk func(node engine.Node)
	walk = func(node engine.Node) {
		if node == nil {
			return
		}
		switch {
		case node.IsMissing():
			issues = append(issues, SyntaxIssue{Kind: "missing", Location: locationFromNode(node)})
		case node.IsError():
			issues = append(issues, SyntaxIssue{Kind: "error", Location: locationFromNode(node)})
		}
		count := node.ChildCount()
		for i := 0; i < count; i++ {
			walk(node.Child(i))
		}
	}
	walk(n)
	return issues
}
