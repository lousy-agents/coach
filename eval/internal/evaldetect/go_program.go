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

func loadGoProgram(root string) (*goProgram, error) {
	modules, err := goModules(root)
	if err != nil {
		return nil, err
	}
	program := &goProgram{fset: token.NewFileSet(), packages: map[string]*goPackage{}, stdlib: importer.Default()}
	err = walkFiles(root, isProductionGo, func(rel string) error {
		return program.addFile(root, rel, modules)
	})
	if err != nil {
		return nil, err
	}
	for _, importPath := range program.importPaths() {
		if _, err := program.Import(importPath); err != nil {
			return nil, err
		}
	}
	return program, nil
}

func isProductionGo(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

func (program *goProgram) addFile(root, rel string, modules []goModule) error {
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

// importPathOf maps a root-relative directory to its import path in the
// innermost module containing it.
func importPathOf(dir string, modules []goModule) (string, bool) {
	best := -1
	for i, module := range modules {
		if module.dir != "." && dir != module.dir && !strings.HasPrefix(dir, module.dir+"/") {
			continue
		}
		if best < 0 || len(module.dir) > len(modules[best].dir) {
			best = i
		}
	}
	if best < 0 {
		return "", false
	}
	module := modules[best]
	rest := strings.TrimPrefix(strings.TrimPrefix(dir, module.dir), "/")
	if module.dir == "." {
		rest = strings.TrimPrefix(dir, ".")
	}
	return path.Join(module.path, rest), true
}

func (program *goProgram) importPaths() []string {
	paths := make([]string, 0, len(program.packages))
	for importPath := range program.packages {
		paths = append(paths, importPath)
	}
	sort.Strings(paths)
	return paths
}

// Import type-checks a local package on first use and hands any other
// import to the standard library's importer.
func (program *goProgram) Import(importPath string) (*types.Package, error) {
	pkg, local := program.packages[importPath]
	if !local {
		return program.stdlib.Import(importPath)
	}
	if pkg.pkg != nil {
		return pkg.pkg, nil
	}
	pkg.info = &types.Info{Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}, Defs: map[*ast.Ident]types.Object{}}
	config := types.Config{Importer: program}
	checked, err := config.Check(importPath, program.fset, pkg.files, pkg.info)
	if err != nil {
		return nil, fmt.Errorf("%w: package %s does not type-check: %v", ErrRefused, importPath, err)
	}
	pkg.pkg = checked
	return checked, nil
}
