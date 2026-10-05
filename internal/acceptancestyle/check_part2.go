package acceptancestyle

import (
	"fmt"
	"go/ast"

	"path/filepath"
	"strconv"
)

// violationForAcceptanceFile reports path's style violation. found is false
// both when path is not an acceptance candidate and when it satisfies the
// rule or is allowlisted -- only a genuine violation sets it.
func violationForAcceptanceFile(root, path, name string) (violation Violation, found bool, err error) {
	if !isAcceptanceTestFile(name) {
		return Violation{}, false, nil
	}
	rel, relErr := filepath.Rel(root, path)
	if relErr != nil {
		rel = path
	}
	rel = filepath.ToSlash(rel)

	if _, ok := allowlist[rel]; ok {
		return Violation{}, false, nil
	}

	ok, reason, checkErr := satisfiesGinkgoStyle(path)
	if checkErr != nil {
		return Violation{}, false, fmt.Errorf("%s: %w", rel, checkErr)
	}
	if ok {
		return Violation{}, false, nil
	}
	return Violation{Path: rel, Reason: reason}, true, nil
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
