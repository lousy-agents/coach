package semantics

import (
	"github.com/lousy-agents/coach/pkg/semantics/internal/engine"
)

func goHasLabel(n engine.Node) bool {
	count := n.ChildCount()
	for i := 0; i < count; i++ {
		if n.Child(i).Kind() == "label_name" {
			return true
		}
	}
	return false
}

func isGoDirectRecursion(call engine.Node, source []byte, funcName string) bool {
	if funcName == "" {
		return false
	}
	fn := call.ChildByFieldName("function")
	if fn == nil || fn.Kind() != "identifier" {
		return false
	}
	return fn.Utf8Text(source) == funcName
}
