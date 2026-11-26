package evalpatterns

import "go/ast"

const (
	entryHTTPHandler = "http_handler"
	entryMethod      = "method"
	entryFunction    = "function"
)

// entryKindOf classifies an exported function by its signature, read from
// syntax alone: an http.Handler-shaped function or ServeHTTP method, any
// other method on an exported type, or a plain function. Anything else is
// no entry, "".
func entryKindOf(file *ast.File, function *ast.FuncDecl) string {
	switch {
	case !function.Name.IsExported():
		return ""
	case isHTTPHandler(file, function):
		return entryHTTPHandler
	case function.Recv == nil:
		return entryFunction
	case exportedReceiver(function.Recv):
		return entryMethod
	}
	return ""
}

func (k *entryKinds) add(kind string) {
	switch kind {
	case entryHTTPHandler:
		k.HTTPHandler++
	case entryMethod:
		k.Method++
	case entryFunction:
		k.Function++
	}
}

func isHTTPHandler(file *ast.File, function *ast.FuncDecl) bool {
	http := importName(file, "net/http")
	if http == "" {
		return false
	}
	types := paramTypes(function.Type.Params)
	return len(types) == 2 && isSelector(types[0], http, "ResponseWriter") && isPointerTo(types[1], http, "Request") &&
		(function.Recv == nil || function.Name.Name == "ServeHTTP")
}

func exportedReceiver(receiver *ast.FieldList) bool {
	expr := receiver.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch base := expr.(type) {
	case *ast.IndexExpr:
		expr = base.X
	case *ast.IndexListExpr:
		expr = base.X
	}
	ident, ok := expr.(*ast.Ident)
	return ok && ident.IsExported()
}
