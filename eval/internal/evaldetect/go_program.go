package evaldetect

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// goPackage is one production package under the root, parsed and
// type-checked.
type goPackage struct {
	importPath string
	dir        string
	files      []*ast.File
	fileNames  []string
	pkg        *types.Package
	info       *types.Info
}

// goProgram is every production Go package under a root, type-checked
// offline: local packages from source, the standard library from its own
// export data. A package that does not type-check refuses the whole program.
type goProgram struct {
	fset     *token.FileSet
	packages map[string]*goPackage
	stdlib   types.Importer
}

func loadGoProgram(root string) (goProgram, error) {
	modules, err := goModules(root)
	if err != nil {
		return goProgram{}, err
	}
	program := goProgram{fset: token.NewFileSet(), packages: map[string]*goPackage{}, stdlib: importer.Default()}
	err = walkFiles(root, isProductionGo, func(rel string) error {
		return program.addFile(root, rel, modules)
	})
	if err != nil {
		return goProgram{}, err
	}
	for _, importPath := range program.importPaths() {
		if _, err := program.Import(importPath); err != nil {
			return goProgram{}, err
		}
	}
	return program, nil
}

func isProductionGo(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func (program goProgram) addFile(root, rel string, modules []goModule) error {
	dir := path.Dir(rel)
	importPath, ok := importPathOf(dir, modules)
	if !ok {
		return nil
	}
	file, err := parser.ParseFile(program.fset, filepath.Join(root, filepath.FromSlash(rel)), nil, 0)
	if err != nil {
		return fmt.Errorf("%w: %s does not parse: %v", ErrRefused, rel, err)
	}
	pkg := program.packages[importPath]
	if pkg == nil {
		pkg = &goPackage{importPath: importPath, dir: dir}
		program.packages[importPath] = pkg
	}
	pkg.files = append(pkg.files, file)
	pkg.fileNames = append(pkg.fileNames, rel)
	return nil
}

func (program goProgram) importPaths() []string {
	paths := make([]string, 0, len(program.packages))
	for importPath := range program.packages {
		paths = append(paths, importPath)
	}
	sort.Strings(paths)
	return paths
}
