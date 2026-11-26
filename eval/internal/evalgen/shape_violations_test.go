package evalgen_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func shapeViolations(t *testing.T, path string, content []byte) []string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, content, 0)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var violations []string
	ast.Inspect(parsed, func(node ast.Node) bool {
		violations = append(violations, nodeViolations(node)...)
		return true
	})
	return violations
}

func nodeViolations(node ast.Node) []string {
	switch typed := node.(type) {
	case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.GoStmt, *ast.DeferStmt:
		return []string{"control flow"}
	case *ast.UnaryExpr:
		return addressOf(typed)
	case *ast.FuncDecl:
		return funcViolations(typed)
	}
	return nil
}

func addressOf(expression *ast.UnaryExpr) []string {
	if expression.Op == token.AND {
		return []string{"address-of"}
	}
	return nil
}

func funcViolations(function *ast.FuncDecl) []string {
	var violations []string
	if strings.HasPrefix(function.Name.Name, "New") {
		violations = append(violations, "constructor "+function.Name.Name)
	}
	if function.Type.Results == nil {
		return violations
	}
	for _, result := range function.Type.Results.List {
		if _, pointer := result.Type.(*ast.StarExpr); pointer {
			violations = append(violations, "pointer return in "+function.Name.Name)
		}
	}
	return violations
}
