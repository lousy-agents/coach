package baseline_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
)

// hotColdHiddenMutationFixtureRoot builds a temp tree with one hot path and
// several cold paths (20 + 3 + 3 + 2) for Story 3 priority-cap round-robin.
func hotColdHiddenMutationFixtureRoot() string {
	GinkgoHelper()
	root := GinkgoT().TempDir()

	writeHiddenMutators(root, "hot", "hot.go", 20)
	writeHiddenMutators(root, "colda", "cold_a.go", 3)
	writeHiddenMutators(root, "coldb", "cold_b.go", 3)
	writeHiddenMutators(root, "coldc", "cold_c.go", 2)
	return root
}

// writeHiddenMutators writes root/path: package pkg with a struct of n string
// fields and n functions that each mutate one field through a pointer
// parameter, i.e. n hidden_input_mutation signals.
func writeHiddenMutators(root, pkg, path string, n int) {
	GinkgoHelper()
	var b strings.Builder
	fmt.Fprintf(&b, "package %s\n\ntype S struct {\n", pkg)
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "\tF%d string\n", i)
	}
	b.WriteString("}\n\n")
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "func M%d(s *S, v string) { s.F%d = v }\n", i, i)
	}
	Expect(os.WriteFile(filepath.Join(root, path), []byte(b.String()), 0o644)).To(Succeed())
}

func hasJudgmentCapDiagnostic(diags []coachapi.JobDiagnostic) (found bool, message string) {
	for _, d := range diags {
		msg := d.Message
		scope := strings.ToLower(d.Scope)
		if strings.Contains(msg, "judgment_cap_omitted") ||
			scope == "judgment_cap" ||
			(strings.Contains(msg, "selected=") && strings.Contains(msg, "omitted=")) {
			return true, d.Message
		}
	}
	return false, ""
}
