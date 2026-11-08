package gitrepo

import (
	"fmt"
	"go/ast"
	"go/types"
)

// Which constant of git_budgets.go an argument names, followed through the aliases of its package.

const maxAliasDepth = 8

// namedBudget follows expression through the aliases of its package to the constant of git_budgets.go it
// names. Inside this package, an exported name is such a constant; in another, it is gitrepo.Name.
func namedBudget(expression ast.Expr, aliases map[string]ast.Expr, insideGitrepo bool, depth int) (string, error) {
	if depth > maxAliasDepth {
		return "", fmt.Errorf("%s is not resolved after %d aliases", types.ExprString(expression), maxAliasDepth)
	}
	switch e := expression.(type) {
	case *ast.SelectorExpr:
		if pkg, isPackage := e.X.(*ast.Ident); isPackage && pkg.Name == "gitrepo" {
			return e.Sel.Name, nil
		}
	case *ast.Ident:
		return namedIdentifier(e, aliases, insideGitrepo, depth)
	}
	return "", fmt.Errorf("%s is not a named budget constant", types.ExprString(expression))
}

func namedIdentifier(identifier *ast.Ident, aliases map[string]ast.Expr, insideGitrepo bool, depth int) (string, error) {
	if insideGitrepo && ast.IsExported(identifier.Name) {
		return identifier.Name, nil
	}
	next, declared := aliases[identifier.Name]
	if !declared {
		return "", fmt.Errorf("%s is not declared at the top of the package", identifier.Name)
	}
	return namedBudget(next, aliases, insideGitrepo, depth+1)
}
