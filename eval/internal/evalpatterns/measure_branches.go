package evalpatterns

import "go/ast"

// branchPoints counts a file's if, for (range included), switch, type
// switch and select statements: the sum Coach's branch density reads.
func branchPoints(file *ast.File) int {
	count := 0
	ast.Inspect(file, func(n ast.Node) bool {
		if isBranch(n) {
			count++
		}
		return true
	})
	return count
}

func isBranch(n ast.Node) bool {
	switch n.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
		return true
	}
	return false
}

// maxNesting is the deepest chain of branch statements in body, where an
// else-if continues its if rather than nesting inside it.
func maxNesting(body ast.Node) int {
	deepest := 0
	ast.Inspect(body, func(n ast.Node) bool {
		if n == body || n == nil {
			return true
		}
		if isBranch(n) {
			deepest = max(deepest, 1+nestedBelow(n))
			return false
		}
		return true
	})
	return deepest
}

// nestedBelow is the nesting inside one branch statement: its bodies, and
// an else-if chain at the statement's own depth.
func nestedBelow(n ast.Node) int {
	ifStmt, ok := n.(*ast.IfStmt)
	if !ok {
		return maxNesting(n)
	}
	inside := maxNesting(ifStmt.Body)
	switch alternative := ifStmt.Else.(type) {
	case *ast.IfStmt:
		inside = max(inside, nestedBelow(alternative))
	case *ast.BlockStmt:
		inside = max(inside, maxNesting(alternative))
	}
	return inside
}
