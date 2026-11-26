package main_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"

	. "github.com/onsi/gomega"
)

func productionShapeViolations(path string, content []byte) []string {
	parsed, err := parser.ParseFile(token.NewFileSet(), path, content, 0)
	Expect(err).NotTo(HaveOccurred(), path)
	var violations []string
	ast.Inspect(parsed, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			violations = append(violations, "control flow")
		case *ast.FuncDecl:
			violations = append(violations, constructorOrPointerReturn(typed)...)
		}
		return true
	})
	return violations
}

func constructorOrPointerReturn(function *ast.FuncDecl) []string {
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

func sourceDirectories(tree map[string][]byte, policy policyFile) map[string]bool {
	directories := map[string]bool{}
	for path := range tree {
		if isProductionGo(path) && inAnyLayer(path, policy) {
			directories[filepath.ToSlash(filepath.Dir(path))] = true
		}
	}
	return directories
}

func isProductionGo(path string) bool {
	return strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go")
}

func inAnyLayer(path string, policy policyFile) bool {
	for _, layer := range policy.Layers {
		if strings.HasPrefix(path, layer.Prefixes[0]+"/") {
			return true
		}
	}
	return false
}
