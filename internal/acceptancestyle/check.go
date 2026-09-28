// Package acceptancestyle mechanically checks that every acceptance_test.go
// and *_acceptance_test.go imports github.com/onsi/ginkgo/v2 and references a
// Ginkgo suite/spec API (RunSpecs, Describe, It, …), except an explicit allowlist.
package acceptancestyle

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"

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
	walk := &acceptanceWalk{root: root}
	err := filepath.WalkDir(root, walk.visit)
	if err != nil {
		return nil, err
	}
	return walk.violations, nil
}

type acceptanceWalk struct {
	root       string
	violations []Violation
}

func (w *acceptanceWalk) visit(path string, d fs.DirEntry, err error) error {
	if err != nil {
		return err
	}
	if d.IsDir() {
		if _, skip := skipDirNames[d.Name()]; skip {
			return fs.SkipDir
		}
		return nil
	}
	violation, found, checkErr := violationForAcceptanceFile(w.root, path, d.Name())
	if checkErr != nil {
		return checkErr
	}
	if found {
		w.violations = append(w.violations, violation)
	}
	return nil
}

// violationForAcceptanceFile reports path's style violation. found is false
// both when path is not an acceptance candidate and when it satisfies the
// rule or is allowlisted -- only a genuine violation sets it.

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
