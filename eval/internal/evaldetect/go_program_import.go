package evaldetect

import (
	"fmt"
	"go/ast"
	"go/types"
)

// Import type-checks a local package on first use and hands any other
// import to the standard library's importer. A local package that imports
// itself, directly or through others, is refused rather than recursed into.
func (program goProgram) Import(importPath string) (*types.Package, error) {
	pkg, local := program.packages[importPath]
	if !local {
		return program.stdlib.Import(importPath)
	}
	if pkg.pkg != nil {
		return pkg.pkg, nil
	}
	if pkg.checking {
		return nil, fmt.Errorf("%w: import cycle through %s", ErrRefused, importPath)
	}
	pkg.checking = true
	defer func() { pkg.checking = false }()
	pkg.info = &types.Info{Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}, Defs: map[*ast.Ident]types.Object{}}
	config := types.Config{Importer: program}
	checked, err := config.Check(importPath, program.fset, pkg.files, pkg.info)
	if err != nil {
		return nil, fmt.Errorf("%w: package %s does not type-check: %v", ErrRefused, importPath, err)
	}
	pkg.pkg = checked
	return checked, nil
}
