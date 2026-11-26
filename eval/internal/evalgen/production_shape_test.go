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
		cell, err := evalgen.CellFromIndex(index)
		if err != nil {
			t.Fatal(err)
		}
		for _, seed := range []int64{0, 3} {
			program, err := evalgen.Generate(cell, seed)
			if err != nil {
				t.Fatalf("cell %d seed %d: %v", index, seed, err)
			}
			for path, content := range program.Tree {
				if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
					continue
				}
				for _, violation := range shapeViolations(t, path, content) {
					t.Errorf("cell %d seed %d: %s: %s", index, seed, path, violation)
				}
			}
		}
	}
}

func shapeViolations(t *testing.T, path string, content []byte) []string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, content, 0)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var violations []string
	ast.Inspect(parsed, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.GoStmt, *ast.DeferStmt:
			violations = append(violations, "control flow")
		case *ast.UnaryExpr:
			if typed.Op == token.AND {
				violations = append(violations, "address-of")
			}
		case *ast.FuncDecl:
			violations = append(violations, funcViolations(typed)...)
		}
		return true
	})
	return violations
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
