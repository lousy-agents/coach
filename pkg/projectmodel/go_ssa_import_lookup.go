package projectmodel

import "go/types"

func importByPath(pkg *types.Package, pkgPath string) *types.Package {
	for _, ip := range pkg.Imports() {
		if ip.Path() == pkgPath {
			return ip
		}
	}
	return nil
}
