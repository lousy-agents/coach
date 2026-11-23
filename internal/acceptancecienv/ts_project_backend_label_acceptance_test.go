package acceptancecienv

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const builtJSOutputs = "js/semantics/node_modules"

var labelArgs = []string{probePackage, "-args", "-ginkgo.label-filter=ts-project-backend"}

// installJSFixture puts a file in the repository's installed js/semantics, and
// a link from it to a file in a directory only the caller can read, as a mise
// or pnpm install links into a store under the caller's HOME.
func installJSFixture(repo string) {
	installed := filepath.Join(repo, builtJSOutputs)
	Expect(os.MkdirAll(filepath.Join(installed, "fixture"), 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(installed, "fixture", "index.js"), []byte("fixture-content"), 0o644)).To(Succeed())
	store, err := os.MkdirTemp("", "acceptance-ci-env-store-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, store)
	Expect(os.Chmod(store, 0o700)).To(Succeed())
	Expect(os.MkdirAll(filepath.Join(store, "linked"), 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(store, "linked", "file.txt"), []byte("linked-content"), 0o644)).To(Succeed())
	Expect(os.Symlink(filepath.Join(store, "linked"), filepath.Join(installed, "linked"))).To(Succeed())
}

var _ = Describe("mise run acceptance-ci-env with the ts-project-backend label (#733 AC7)", func() {
	It("#733 AC7: when js/semantics is not installed, refuses with exit 2, naming it and `mise run js-install`, and runs no test", func() {
		run := runTaskIn(repositoryCopy(), hostGitVersion(), labelArgs)

		Expect(run.exitCode).To(Equal(2), "output:\n%s", run.output())
		Expect(run.output()).To(ContainSubstring("the ts-project-backend label needs " + builtJSOutputs))
		Expect(run.output()).To(ContainSubstring("mise run js-install"))
		Expect(run.output()).NotTo(ContainSubstring("flag provided but not defined"), "the refusal must come before any test is run")
		Expect(run.probe).To(BeNil(), "no test may run without the compiler the label needs")
	})

	It("#733 AC7: when js/semantics is installed, the tests read it, its links included, and the task says how it provided it", func() {
		repo := repositoryCopy()
		installJSFixture(repo)

		run := runTaskIn(repo, hostGitVersion(), labelArgs)

		Expect(run.exitCode).To(Equal(0), "output:\n%s", run.output())
		Expect(run.output()).To(MatchRegexp(`(copied|uses) the installed js/semantics/node_modules`))
		Expect(run.probe).NotTo(BeNil(), "output:\n%s", run.output())
		Expect(run.probe.JSInstalled).To(Equal(map[string]string{
			"fixture/index.js": "fixture-content",
			"linked/file.txt":  "linked-content",
		}), "the tests must read the installed files, and the files behind its links")
	})
})
