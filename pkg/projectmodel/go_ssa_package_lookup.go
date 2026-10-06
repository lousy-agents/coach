package projectmodel

import (
	"go/types"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
)

// typesPackageByPath returns the *types.Package for pkgPath from prog if
// ssautil created an SSA package for it, otherwise from the initial
// packages' type-checker import graph. LoadSyntax does not populate
// packages.Package.Types on dependencies, so prog.ImportedPackage can be
// nil for net/http even when the fixture type-checked against it.
func typesPackageByPath(prog *ssa.Program, pkgs []*packages.Package, pkgPath string) *types.Package {
	if prog != nil {
		if pkg := prog.ImportedPackage(pkgPath); pkg != nil {
			return pkg.Pkg
		}
	}
	for _, p := range pkgs {
		if p.Types == nil {
			continue
		}
		if p.PkgPath == pkgPath {
			return p.Types
		}
		if pkg := importByPath(p.Types, pkgPath); pkg != nil {
			return pkg
		}
	}
	return nil
}
