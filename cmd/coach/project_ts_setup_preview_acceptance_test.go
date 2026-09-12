package main

import (
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

// BuildSetupPreview is a pure function over an already-selected SetupChoice,
// the package-manager kind checkPackageManager already classified, and the
// manifest's directory -- it makes no filesystem or network call and spawns
// nothing. The exported Go function is therefore the most meaningful public
// boundary available for this behavior today; confirm/execute wiring (and
// any CLI-facing rendering of a preview) is a later task.

var _ = Describe("codesignalcli.BuildSetupPreview", func() {
	When("the selected setup choice is project-package, resolved to npm", func() {
		It("discloses the exact npm argv, working directory, lockfile-only change, network reach, script suppression, and a bounded timeout, without invoking any subprocess (AC-SET-2)", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0","packageManager":"npm@11.2.0"}`+"\n")
			commitFile(repo, "package-lock.json", `{"name":"example","lockfileVersion":3}`+"\n")

			// Strip every npm/pnpm/bun/node/mise executable from PATH before
			// producing the preview: if BuildSetupPreview spawned or even
			// looked up any of them, a bug that made it do so would have
			// nothing to find here.
			GinkgoT().Setenv("PATH", pathExcludingExecutables("npm", "pnpm", "bun", "node", "mise"))
			probe := exec.Command("sh", "-c", "command -v npm || command -v pnpm || command -v bun")
			Expect(probe.Run()).To(HaveOccurred(), "expected npm/pnpm/bun to be unreachable on the stripped PATH used for this spec")

			preview, err := codesignalcli.BuildSetupPreview(
				codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage},
				"npm",
				repo,
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(preview.Executable).To(Equal("npm"), "must be the bare binary name, not a resolved absolute path -- resolving/confining the executable is a later task")
			Expect(preview.Args).To(Equal([]string{"ci", "--ignore-scripts"}))
			Expect(preview.WorkingDirectory).To(Equal(repo), "must be the directory containing the manifest that owns the selected origin")
			Expect(preview.ExpectedChanges).To(ContainSubstring("package-lock.json"), "must name the file this command may rewrite")
			Expect(preview.ExpectedChanges).To(ContainSubstring("package.json"), "must state that package.json's declared dependencies are untouched")
			Expect(preview.ExpectedChanges).To(ContainSubstring("never"), "npm ci never adds/removes/updates declared dependencies")
			Expect(preview.NetworkDisclosure).To(And(ContainSubstring("network"), ContainSubstring("registry")), "must truthfully disclose that this command may reach the package registry -- none of the matrix commands are offline")
			Expect(preview.ScriptSuppressionPolicy).To(ContainSubstring("--ignore-scripts"), "must name the flag suppressing lifecycle scripts")
			Expect(preview.Timeout).To(BeNumerically(">", 0), "must disclose a bounded, non-zero timeout")
			Expect(preview.Timeout).To(BeNumerically("<=", 10*time.Minute))
		})
	})

	DescribeTable("sources the exact frozen argv per package-manager kind (SA-280-012)",
		func(kind, wantExecutable string, wantArgs []string) {
			preview, err := codesignalcli.BuildSetupPreview(
				codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage},
				kind,
				"/tmp/example-root",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(preview.Executable).To(Equal(wantExecutable))
			Expect(preview.Args).To(Equal(wantArgs))
		},
		Entry("npm", "npm", "npm", []string{"ci", "--ignore-scripts"}),
		Entry("pnpm", "pnpm", "pnpm", []string{"install", "--frozen-lockfile", "--ignore-scripts"}),
		Entry("bun", "bun", "bun", []string{"install", "--frozen-lockfile", "--ignore-scripts"}),
	)

	When("the selected choice is not project-package", func() {
		It("fails closed rather than guessing a command", func() {
			_, err := codesignalcli.BuildSetupPreview(
				codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectMise},
				"npm",
				"/tmp/example-root",
			)
			Expect(err).To(HaveOccurred())
		})
	})

	When("packageManagerKind names a manager outside the frozen adapter matrix", func() {
		It("fails closed rather than guessing a command", func() {
			_, err := codesignalcli.BuildSetupPreview(
				codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage},
				"yarn",
				"/tmp/example-root",
			)
			Expect(err).To(HaveOccurred())
		})
	})
})
