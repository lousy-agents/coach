package gitrepo

import (
	"go/ast"
	"go/token"
	"maps"
	"testing"
)

// The names at the top of a package, and what each is given.

// valuesOfSpec is the expression each name of one const or var spec is given.
func valuesOfSpec(spec *ast.ValueSpec) map[string]ast.Expr {
	values := map[string]ast.Expr{}
	for i, name := range spec.Names {
		if i < len(spec.Values) {
			values[name.Name] = spec.Values[i]
		}
	}
	return values
}

// valuesOfDeclaration is the names a const or var declaration gives; any other declaration gives none.
func valuesOfDeclaration(declaration ast.Decl) map[string]ast.Expr {
	values := map[string]ast.Expr{}
	general, isGeneral := declaration.(*ast.GenDecl)
	if !isGeneral || (general.Tok != token.CONST && general.Tok != token.VAR) {
		return values
	}
	for _, spec := range general.Specs {
		maps.Copy(values, valuesOfSpec(spec.(*ast.ValueSpec)))
	}
	return values
}

// topLevelValues is the expression each name of a const or var declaration at the top of file is given.
func topLevelValues(file *ast.File) map[string]ast.Expr {
	values := map[string]ast.Expr{}
	for _, declaration := range file.Decls {
		maps.Copy(values, valuesOfDeclaration(declaration))
	}
	return values
}

// aliasesOf is what the names at the top of the package in dir are given.
func aliasesOf(t *testing.T, dir string) map[string]ast.Expr {
	t.Helper()
	aliases := map[string]ast.Expr{}
	for _, path := range nonTestFiles(t, dir) {
		maps.Copy(aliases, topLevelValues(parseFile(t, path)))
	}
	return aliases
}
