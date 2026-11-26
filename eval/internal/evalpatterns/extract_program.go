package evalpatterns

import "go/ast"

// measureProgram records what only the whole checkout shows: its call depth,
// which needs the module path, and its layers, which need the policy.
func (p *profile) measureProgram(root string, packages map[string][]*ast.File) error {
	shape, err := layersOf(root, sortedDirs(packages))
	if err != nil {
		return err
	}
	module, err := modulePath(root)
	if err != nil {
		return err
	}
	p.recordDepths(entryDepths(module, packages))
	p.recordLayers(shape)
	return nil
}
