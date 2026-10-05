package acceptancestyle

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
)

const ginkgoImportPath = "github.com/onsi/ginkgo/v2"

func satisfiesGinkgoStyle(path string) (ok bool, reason string, err error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return false, "", err
	}

	importNames, hasNonBlank, hasBlank := ginkgoImportNames(f)
	if !hasNonBlank && hasBlank {
		return false, "blank import of " + ginkgoImportPath + " is not enough; reference RunSpecs/Describe/It (or be allowlisted)", nil
	}
	if !hasNonBlank {
		return false, "must import " + ginkgoImportPath + " and reference RunSpecs/Describe/It (or be allowlisted)", nil
	}

	if !referencesGinkgoSpec(f, importNames) {
		return false, "must reference a Ginkgo suite/spec API (RunSpecs, Describe, It, …); import alone is not enough", nil
	}
	return true, "", nil
}

func ginkgoImportNames(f *ast.File) (names map[string]struct{}, hasNonBlank, hasBlank bool) {
	names = make(map[string]struct{})
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != ginkgoImportPath {
			continue
		}
		if imp.Name != nil && imp.Name.Name == "_" {
			hasBlank = true
			continue
		}
		hasNonBlank = true
		switch {
		case imp.Name == nil:
			names["ginkgo"] = struct{}{}
		case imp.Name.Name == ".":
			names["."] = struct{}{}
		default:
			names[imp.Name.Name] = struct{}{}
		}
	}
	return names, hasNonBlank, hasBlank
}
