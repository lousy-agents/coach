// Package acceptancestyle mechanically checks that every acceptance_test.go
// and *_acceptance_test.go imports github.com/onsi/ginkgo/v2 and references a
// Ginkgo suite/spec API (RunSpecs, Describe, It, …), except an explicit allowlist.
package acceptancestyle

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

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
