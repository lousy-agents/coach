package evalpatterns

import (
	"go/ast"
	"strings"
	"unicode"
)

const (
	constructorAssignsOnly = "assigns_only"
	constructorDoesWork    = "does_work"
)

// outwardCall is one entry of the closed list of APIs that reach outside
// the process (the chief's ruling on #872, matching #876). Name is a
// package-level function, a selector path below the package such as
// DefaultClient.Do, a prefix ending in *, or * for the whole package.
type outwardCall struct {
	path string
	name string
}

var outwardCalls = []outwardCall{
	{"os", "Getenv"}, {"os", "LookupEnv"}, {"os", "Open*"}, {"os", "Create"},
	{"os", "ReadFile"}, {"os", "WriteFile"}, {"os", "Mkdir*"},
	{"os/exec", "*"},
	{"net", "Dial*"}, {"net", "Listen*"},
	{"net/http", "Get"}, {"net/http", "Post"}, {"net/http", "Head"},
	{"net/http", "DefaultClient.Do"}, {"net/http", "ListenAndServe*"},
	{"database/sql", "Open"},
	{"syscall", "*"},
}

// constructorShapeOf classifies a New… function: does_work when its body
// calls an outward API (Hevery's "constructor does real work"),
// assigns_only otherwise. Any other function is no constructor, "".
func constructorShapeOf(file *ast.File, function *ast.FuncDecl) string {
	switch {
	case !isConstructorName(function):
		return ""
	case callsOutward(file, function.Body):
		return constructorDoesWork
	}
	return constructorAssignsOnly
}

func (c *constructorShapes) add(shape string) {
	switch shape {
	case constructorAssignsOnly:
		c.AssignsOnly++
	case constructorDoesWork:
		c.DoesWork++
	}
}

func isConstructorName(function *ast.FuncDecl) bool {
	rest, ok := strings.CutPrefix(function.Name.Name, "New")
	return ok && function.Recv == nil && function.Type.Results != nil &&
		(rest == "" || unicode.IsUpper([]rune(rest)[0]))
}

func callsOutward(file *ast.File, body *ast.BlockStmt) bool {
	outward := false
	ast.Inspect(body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && isOutward(file, call.Fun) {
			outward = true
		}
		return !outward
	})
	return outward
}

func isOutward(file *ast.File, fun ast.Expr) bool {
	pkg, rest, ok := packageSelector(fun)
	if !ok {
		return false
	}
	for _, entry := range outwardCalls {
		if importName(file, entry.path) == pkg && matchesName(entry.name, rest) {
			return true
		}
	}
	return false
}

// packageSelector splits pkg.A.B into pkg and "A.B" when the expression
// starts at a bare identifier.
func packageSelector(expr ast.Expr) (string, string, bool) {
	var names []string
	for {
		selector, ok := expr.(*ast.SelectorExpr)
		if !ok {
			break
		}
		names = append([]string{selector.Sel.Name}, names...)
		expr = selector.X
	}
	ident, ok := expr.(*ast.Ident)
	if !ok || len(names) == 0 {
		return "", "", false
	}
	return ident.Name, strings.Join(names, "."), true
}

func matchesName(pattern, name string) bool {
	if prefix, ok := strings.CutSuffix(pattern, "*"); ok {
		return strings.HasPrefix(name, prefix)
	}
	return pattern == name
}
