package gitrepo

import (
	"go/ast"
	"go/token"
	"strconv"
)

// Which calls of the source ask git a question, and which subcommand they ask.

// appendAsked adds the subcommand that node asks, if it is a call that asks one.
func appendAsked(asked []string, node ast.Node) []string {
	call, isCall := node.(*ast.CallExpr)
	if !isCall {
		return asked
	}
	if subcommand, ok := askedSubcommand(call); ok {
		return append(asked, subcommand)
	}
	return asked
}

// askedSubcommand is the string literal that follows the directory in a call of RunBytes, Run or run.
func askedSubcommand(call *ast.CallExpr) (string, bool) {
	var name string
	switch function := call.Fun.(type) {
	case *ast.Ident:
		name = function.Name
	case *ast.SelectorExpr:
		name = function.Sel.Name
	}
	if (name != "RunBytes" && name != "Run" && name != "run") || len(call.Args) < 2 {
		return "", false
	}
	literal, ok := call.Args[1].(*ast.BasicLit)
	if !ok || literal.Kind != token.STRING {
		return "", false
	}
	subcommand, err := strconv.Unquote(literal.Value)
	return subcommand, err == nil
}
