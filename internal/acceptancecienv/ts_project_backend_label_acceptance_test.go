package acceptancecienv

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const builtJSOutputs = "js/semantics/node_modules"

var _ = Describe("mise run acceptance-ci-env with the ts-project-backend label (#733 AC7)", func() {
	It("#733 AC7: copies js/semantics' built outputs into the run and says so when the caller has built them, and otherwise refuses, naming `mise run js-install`, and runs no test", func() {
		run := runTaskWith(hostGitVersion(), []string{probePackage, "-args", "-ginkgo.label-filter=ts-project-backend"})

		if _, err := os.Stat(filepath.Join(repoRoot, builtJSOutputs)); err == nil {
			Expect(run.output()).To(ContainSubstring("copied js/semantics' built outputs"), "output:\n%s", run.output())
			return
		}
		Expect(run.exitCode).NotTo(Equal(0), "output:\n%s", run.output())
		Expect(run.output()).To(ContainSubstring(builtJSOutputs))
		Expect(run.output()).To(ContainSubstring("mise run js-install"))
		Expect(run.probe).To(BeNil(), "no test may run without the compiler the label needs")
	})
})
