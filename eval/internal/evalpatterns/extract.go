package evalpatterns

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
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
		measureFile(&p, fset, file)
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

// productionGoFiles lists, sorted, the slash-separated paths of the .go
// files under root that a build would compile: no _test.go file, and
// nothing under testdata, vendor or a directory whose name starts with . or
// _, which the go tool ignores too.
func productionGoFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			if path != root && ignoredDir(name) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			rel, _ := filepath.Rel(root, path)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(files)
	return files, err
}

func ignoredDir(name string) bool {
	return name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func measureFile(p *profile, fset *token.FileSet, file *ast.File) {
	p.Shapes.BranchDensity.add(branchPoints(file))
	for _, decl := range file.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if !ok || function.Body == nil {
			continue
		}
		p.Shapes.Nesting.add(maxNesting(function.Body))
		countEntry(&p.Dimensions.Entry, file, function)
		countConstructor(&p.Shapes.Constructor, function)
	}
	for _, stat := range cognitiveComplexity(fset, file) {
		p.Shapes.Cognitive.add(stat)
	}
}
