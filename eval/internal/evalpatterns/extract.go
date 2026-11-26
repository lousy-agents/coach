package evalpatterns

import (
	"fmt"
	"go/ast"
	"go/token"
)

// ExtractProfile measures the production Go files of the checkout at root
// with go/parser alone, and returns the entry's profile as canonical JSON.
// It runs no command and writes nothing. A file that does not parse
// refuses the entry with exit 3, and a policy that is not JSON with exit 2.
func ExtractProfile(reference Reference, root string) ([]byte, error) {
	if err := reference.Check(); err != nil {
		return nil, err
	}
	files, err := productionGoFiles(root)
	if err != nil {
		return nil, reference.refuse(fmt.Sprintf("the checkout cannot be read: %v", checkoutError(root, err)))
	}
	p := newProfile(reference)
	packages, err := p.measureFiles(reference, root, files)
	if err != nil {
		return nil, err
	}
	if err := p.measureProgram(root, packages); err != nil {
		return nil, reference.refuse(err.Error())
	}
	raw, err := sealed(p)
	if err != nil {
		return nil, err
	}
	if err := ValidateProfile(raw); err != nil {
		return nil, fmt.Errorf("reference %q: the profile does not validate: %w", reference.Label, err)
	}
	return raw, nil
}

func (p *profile) measureFile(fset *token.FileSet, file *ast.File) {
	p.Shapes.BranchDensity.add(branchPoints(file))
	for _, function := range functionsWithBodies(file) {
		p.Shapes.Nesting.add(maxNesting(function.Body))
		p.Dimensions.Entry.add(entryKindOf(file, function))
		p.Shapes.Constructor.add(constructorShapeOf(file, function))
	}
	for _, score := range cognitiveComplexity(fset, file) {
		p.Shapes.Cognitive.add(score)
	}
}

func functionsWithBodies(file *ast.File) []*ast.FuncDecl {
	var functions []*ast.FuncDecl
	for _, decl := range file.Decls {
		if function, ok := decl.(*ast.FuncDecl); ok && function.Body != nil {
			functions = append(functions, function)
		}
	}
	return functions
}
