package evalpatterns

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
)

// ExtractProfile measures the production Go files of the checkout at root
// with go/parser alone, and returns the entry's profile as canonical JSON.
// It runs no command and writes nothing. A file that does not parse
// refuses the entry with exit 3.
func ExtractProfile(reference Reference, root string) ([]byte, error) {
	if err := reference.Check(); err != nil {
		return nil, err
	}
	files, err := productionGoFiles(root)
	if err != nil {
		return nil, reference.refuse(fmt.Sprintf("checkout %s cannot be read: %v", root, err))
	}
	p := newProfile(reference)
	fset := token.NewFileSet()
	for _, rel := range files {
		file, err := parser.ParseFile(fset, filepath.Join(root, filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, Refusal{Label: reference.Label, Exit: AnalysisFailed, Reason: fmt.Sprintf("file %s does not parse: %v", rel, err)}
		}
		p.measureFile(fset, file)
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
