package evaldetect

import "go/types"

// isSourceShaped reports whether a function can start a bypass: exported,
// and for a method, on an exported type.
func isSourceShaped(function *types.Func) bool {
	if !function.Exported() {
		return false
	}
	receiver := function.Type().(*types.Signature).Recv()
	if receiver == nil {
		return true
	}
	named, ok := derefNamed(receiver.Type())
	return ok && named.Obj().Exported()
}

func derefNamed(t types.Type) (*types.Named, bool) {
	if pointer, ok := t.(*types.Pointer); ok {
		t = pointer.Elem()
	}
	named, ok := t.(*types.Named)
	return named, ok
}
