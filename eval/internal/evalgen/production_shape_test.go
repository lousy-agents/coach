package evalgen_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// The generated base has to stay below every Coach threshold by construction, so no entry kind, direction or
// field type may add a branch, a constructor or a pointer return.
func TestEveryCellEmitsBranchFreeProductionCodeWithoutPointerReturns(t *testing.T) {
	for index := 0; index < evalgen.CellCount; index++ {
		for _, seed := range []int64{0, 3} {
			for _, violation := range violationsOfCell(t, index, seed) {
				t.Errorf("cell %d seed %d: %s", index, seed, violation)
			}
		}
	}
}

func violationsOfCell(t *testing.T, index int, seed int64) []string {
	t.Helper()
	cell, err := evalgen.CellFromIndex(index)
	if err != nil {
		t.Fatal(err)
	}
	program, err := evalgen.Generate(cell, seed)
	if err != nil {
		t.Fatalf("cell %d seed %d: %v", index, seed, err)
	}
	var violations []string
	for path, content := range program.Tree {
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			for _, violation := range shapeViolations(t, path, content) {
				violations = append(violations, path+": "+violation)
			}
		}
	}
	return violations
}

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
