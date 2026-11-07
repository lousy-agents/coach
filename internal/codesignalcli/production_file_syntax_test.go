package codesignalcli

import (
	"go/ast"
	"go/token"
	"strconv"
)

const gitrepoImportPath = modulePrefix + "/gitrepo"

func (f productionFile) importsGitrepo() bool {
	for _, spec := range f.syntax.Imports {
		if path, err := strconv.Unquote(spec.Path.Value); err == nil && path == gitrepoImportPath {
			return true
		}
	}
	return false
}

// envAssignments are the places where the file assigns the Env field of something.
func (f productionFile) envAssignments() []string {
	var places []string
	ast.Inspect(f.syntax, func(node ast.Node) bool {
		if assign, ok := node.(*ast.AssignStmt); ok {
			places = append(places, f.envTargets(assign)...)
		}
		return true
	})
	return places
}

func (f productionFile) envTargets(assign *ast.AssignStmt) []string {
	var places []string
	for _, target := range assign.Lhs {
		if selector, ok := target.(*ast.SelectorExpr); ok && selector.Sel.Name == "Env" {
			places = append(places, f.fset.Position(selector.Pos()).String())
		}
	}
	return places
}

// gitSpawns are the places where the file starts a process whose program is the literal "git".
func (f productionFile) gitSpawns() []string {
	var places []string
	ast.Inspect(f.syntax, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok && namesGitAsProgram(call) {
			places = append(places, f.fset.Position(call.Pos()).String())
		}
		return true
	})
	return places
}

// namesGitAsProgram is true for exec.Command("git", ...) and exec.CommandContext(ctx, "git", ...).
func namesGitAsProgram(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch selector.Sel.Name {
	case "Command":
		return isGitLiteral(call.Args, 0)
	case "CommandContext":
		return isGitLiteral(call.Args, 1)
	}
	return false
}

func isGitLiteral(args []ast.Expr, index int) bool {
	if len(args) <= index {
		return false
	}
	literal, ok := args[index].(*ast.BasicLit)
	return ok && literal.Kind == token.STRING && literal.Value == `"git"`
}
