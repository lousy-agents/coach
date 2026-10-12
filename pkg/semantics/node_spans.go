package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

// locationFromNode converts a node's span into our Location type,
// preserving Tree-sitter's 0-based byte/row/col values verbatim.
func locationFromNode(n engine.Node) Location {
	startRow, startCol := n.StartPoint()
	endRow, endCol := n.EndPoint()
	return Location{
		StartByte: n.StartByte(),
		EndByte:   n.EndByte(),
		StartRow:  startRow,
		StartCol:  startCol,
		EndRow:    endRow,
		EndCol:    endCol,
	}
}

func sameNodeSpan(a, b engine.Node) bool {
	return a.StartByte() == b.StartByte() && a.EndByte() == b.EndByte() && a.Kind() == b.Kind()
}
