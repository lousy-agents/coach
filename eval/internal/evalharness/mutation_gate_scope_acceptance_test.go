package evalharness_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
	"github.com/lousy-agents/coach/eval/internal/evalharness"
)

// reportingGremlins is a Gremlins that finds five mutants in the file it is given, none of them detected.
func reportingGremlins(file string) evalharness.Gremlins {
	script := `#!/bin/sh
out=""
while [ $# -gt 0 ]; do
  if [ "$1" = "--output" ]; then out="$2"; fi
  shift
done
printf '{"files":[{"file_name":"` + file + `","mutations":[{"status":"LIVED"},{"status":"LIVED"},{"status":"LIVED"},{"status":"LIVED"},{"status":"LIVED"}]}]}' > "$out"
`
	binary := filepath.Join(GinkgoT().TempDir(), "gremlins")
	Expect(os.WriteFile(binary, []byte(script), 0o755)).To(Succeed())
	return evalharness.Gremlins{Binary: binary, Version: evalharness.PinnedGremlinsVersion}
}

var _ = Describe("the mutation gate's scope as the gate applies it (design §6, step 6)", func() {
	transitive := func() evalgen.Program {
		return scopedProgram(map[string]string{
			"go.mod":                          scopeModule,
			"internal/api/entry/entry.go":     "package entry\n\nimport \"example.com/shop/internal/service\"\n\nfunc Run() { service.Do() }\n",
			"internal/service/service.go":     "package service\n\nimport \"example.com/shop/internal/shared\"\n\nfunc Do() { shared.Help() }\n",
			"internal/shared/shared.go":       "package shared\n\nfunc Help() {}\n",
			"internal/unrelated/unrelated.go": "package unrelated\n\nfunc Other() {}\n",
		})
	}

	It("judges a mutant in a package the api surface reaches only through another, and rejects when it lives (AC-24)", func() {
		rejected, err := evalharness.JudgeMutation(reportingGremlins("internal/shared/shared.go"), GinkgoT().TempDir(), transitive())

		Expect(err).NotTo(HaveOccurred())
		Expect(rejected).To(Equal(evalgen.StepMutation))
	})

	It("does not judge a mutant in a package nothing reachable imports, which is a vacuous pass (AC-24)", func() {
		rejected, err := evalharness.JudgeMutation(reportingGremlins("internal/unrelated/unrelated.go"), GinkgoT().TempDir(), transitive())

		Expect(err).NotTo(HaveOccurred())
		Expect(rejected).To(BeEmpty())
	})

	It("judges a mutant in the api surface's own package (AC-24)", func() {
		rejected, err := evalharness.JudgeMutation(reportingGremlins("internal/api/entry/entry.go"), GinkgoT().TempDir(), transitive())

		Expect(err).NotTo(HaveOccurred())
		Expect(rejected).To(Equal(evalgen.StepMutation))
	})
})
