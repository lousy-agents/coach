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
// exceptions documented in AGENTS.md.
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
		violation, found, checkErr := violationForAcceptanceFile(root, path, d.Name())
		if checkErr != nil {
			return checkErr
		}
		if found {
			violations = append(violations, violation)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return violations, nil
}

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

func satisfiesGinkgoStyle(path string) (ok bool, reason string, err error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return false, "", err
	}

	importNames, hasNonBlank, hasBlank := ginkgoImportNames(f)
	if !hasNonBlank {
		if hasBlank {
			return false, "blank import of " + ginkgoImportPath + " is not enough; reference RunSpecs/Describe/It (or be allowlisted)", nil
		}
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
		if callInvokesGinkgoSpecIdent(call.Fun, importNames) {
			found = true
		}
		return true
	})
	return found
}

func callInvokesGinkgoSpecIdent(fun ast.Expr, importNames map[string]struct{}) bool {
	switch fun := fun.(type) {
	case *ast.Ident:
		if _, dot := importNames["."]; !dot {
			return false
		}
		_, ok := ginkgoSpecIdents[fun.Name]
		return ok
	case *ast.SelectorExpr:
		pkg, ok := fun.X.(*ast.Ident)
		if !ok {
			return false
		}
		if _, ok := importNames[pkg.Name]; !ok {
			return false
		}
		_, ok = ginkgoSpecIdents[fun.Sel.Name]
		return ok
	default:
		return false
	}
}
