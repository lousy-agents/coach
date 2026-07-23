// Package acceptancestyle mechanically checks that every acceptance_test.go
// and *_acceptance_test.go imports github.com/onsi/ginkgo/v2 and references a
// Ginkgo suite/spec API (RunSpecs, Describe, It, …), except an explicit allowlist.
package acceptancestyle

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
)

const ginkgoImportPath = "github.com/onsi/ginkgo/v2"

// Paths relative to the scan root (repo root). Intentional stdlib Test*Acceptance
// exceptions documented in AGENTS.md — exemptions stay in-source until the
// allowlist grows enough to warrant a file-local marker.
var allowlist = map[string]struct{}{
	// Thin stdlib wrapper around queueconformance.Run — not a behavioral Ginkgo suite.
	"internal/acceptanceharness/queueconformance/acceptance_test.go": {},
	// Build-tagged thinproof Compose proof uses stdlib testing by design.
	"internal/acceptanceharness/thinproof/compose_acceptance_test.go": {},
}

var skipDirNames = map[string]struct{}{
	".git":         {},
	"vendor":       {},
	"node_modules": {},
}

// ginkgoSpecIdents are call names that prove real suite/spec usage (not a blank import).
var ginkgoSpecIdents = map[string]struct{}{
	"RunSpecs":      {},
	"Describe":      {},
	"DescribeTable": {},
	"FDescribe":     {},
	"PDescribe":     {},
	"Context":       {},
	"FContext":      {},
	"PContext":      {},
	"When":          {},
	"FWhen":         {},
	"PWhen":         {},
	"It":            {},
	"FIt":           {},
	"PIt":           {},
	"Specify":       {},
	"FSpecify":      {},
	"PSpecify":      {},
	"Entry":         {},
	"FEntry":        {},
	"PEntry":        {},
}

// Violation is one acceptance candidate file that fails the ginkgo style rule.
type Violation struct {
	Path   string
	Reason string
}

func isAcceptanceTestFile(name string) bool {
	return name == "acceptance_test.go" || strings.HasSuffix(name, "_acceptance_test.go")
}

// Check walks root for acceptance_test.go and *_acceptance_test.go files and
// returns violations for any that are not allowlisted and do not both import
// ginkgo/v2 (non-blank) and reference a Ginkgo suite/spec API. Paths in
// Violation.Path are slash-separated and relative to root when possible.
func Check(root string) ([]Violation, error) {
	var violations []Violation
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if _, skip := skipDirNames[d.Name()]; skip {
				return fs.SkipDir
			}
			return nil
		}
		if !isAcceptanceTestFile(d.Name()) {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			rel = path
		}
		rel = filepath.ToSlash(rel)

		if _, ok := allowlist[rel]; ok {
			return nil
		}

		ok, reason, checkErr := satisfiesGinkgoStyle(path)
		if checkErr != nil {
			return fmt.Errorf("%s: %w", rel, checkErr)
		}
		if !ok {
			violations = append(violations, Violation{Path: rel, Reason: reason})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return violations, nil
}

func satisfiesGinkgoStyle(path string) (ok bool, reason string, err error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return false, "", err
	}

	importNames, hasNonBlank := ginkgoImportNames(f)
	if !hasNonBlank {
		if hasBlankGinkgoImport(f) {
			return false, "blank import of " + ginkgoImportPath + " is not enough; reference RunSpecs/Describe/It (or be allowlisted)", nil
		}
		return false, "must import " + ginkgoImportPath + " and reference RunSpecs/Describe/It (or be allowlisted)", nil
	}
	if !referencesGinkgoSpec(f, importNames) {
		return false, "must reference a Ginkgo suite/spec API (RunSpecs, Describe, It, …); import alone is not enough", nil
	}
	return true, "", nil
}

func ginkgoImportNames(f *ast.File) (names map[string]struct{}, hasNonBlank bool) {
	names = make(map[string]struct{})
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != ginkgoImportPath {
			continue
		}
		if imp.Name != nil && imp.Name.Name == "_" {
			continue
		}
		hasNonBlank = true
		switch {
		case imp.Name == nil:
			// Default name is the last path element ("ginkgo"), but this repo
			// almost always uses a dot import; still accept the default.
			names["ginkgo"] = struct{}{}
		case imp.Name.Name == ".":
			names["."] = struct{}{}
		default:
			names[imp.Name.Name] = struct{}{}
		}
	}
	return names, hasNonBlank
}

func hasBlankGinkgoImport(f *ast.File) bool {
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || path != ginkgoImportPath {
			continue
		}
		if imp.Name != nil && imp.Name.Name == "_" {
			return true
		}
	}
	return false
}

func referencesGinkgoSpec(f *ast.File, importNames map[string]struct{}) bool {
	found := false
	ast.Inspect(f, func(n ast.Node) bool {
		if found {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch fun := call.Fun.(type) {
		case *ast.Ident:
			if _, dot := importNames["."]; !dot {
				return true
			}
			if _, ok := ginkgoSpecIdents[fun.Name]; ok {
				found = true
			}
		case *ast.SelectorExpr:
			pkg, ok := fun.X.(*ast.Ident)
			if !ok {
				return true
			}
			if _, ok := importNames[pkg.Name]; !ok {
				return true
			}
			if _, ok := ginkgoSpecIdents[fun.Sel.Name]; ok {
				found = true
			}
		}
		return true
	})
	return found
}
