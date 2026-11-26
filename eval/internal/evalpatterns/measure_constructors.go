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
