package main

import (
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

// BuildSetupPreview is a pure function over an already-selected SetupChoice,
// the checks.package_manager entry checkPackageManager already produced, and
// the manifest's directory -- it makes no filesystem or network call and
// spawns nothing. The exported Go function is therefore the most meaningful
// public boundary available for this behavior today; confirm/execute wiring
// (ExecuteSetup, project_ts_setup_execute.go) exists in this same package,
// but no CLI-facing rendering of a preview exists yet.

// projectPackageManager is the passing checks.package_manager a preview or
// execution spec hands BuildSetupPreview when the classification itself is
// not what that spec exercises.
func projectPackageManager(kind string) codesignalcli.ReadinessCheck {
	return codesignalcli.ReadinessCheck{State: codesignalcli.ReadinessPass, Kind: kind}
}

var _ = Describe("codesignalcli.BuildSetupPreview", func() {
	When("the selected setup choice is project-package, resolved to npm", func() {
		It("discloses the exact npm argv, working directory, node_modules effect, network reach, script suppression, and a bounded timeout, without invoking any subprocess (AC-SET-2)", func() {
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
				projectPackageManager("npm"),
				repo,
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(preview.Executable).To(Equal("npm"), "must be the bare binary name, not a resolved absolute path -- ExecuteSetup resolves it through the child's confined PATH (setupExecutionEnv)")
			Expect(preview.Args).To(Equal([]string{"ci", "--ignore-scripts"}))
			Expect(preview.WorkingDirectory).To(Equal(repo), "must be the directory containing the manifest that owns the selected origin")
			Expect(preview.ExpectedChanges).To(ContainSubstring("node_modules"), "npm ci's actual on-disk mutation is node_modules -- an existing node_modules is removed and recreated")
			Expect(preview.ExpectedChanges).To(ContainSubstring("package-lock.json"), "must name the lockfile this command reads and never writes")
			Expect(preview.ExpectedChanges).NotTo(
				MatchRegexp(setupPreviewLockfileRewriteClaimPattern),
				"npm ci cannot rewrite package-lock.json in place -- it fails instead when package.json and the lockfile disagree",
			)
			Expect(preview.NetworkDisclosure).To(And(ContainSubstring("network"), ContainSubstring("registry")), "must truthfully disclose that this command may reach the package registry -- none of the matrix commands are offline")
			Expect(preview.ScriptSuppressionPolicy).To(ContainSubstring("--ignore-scripts"), "must name the flag suppressing lifecycle scripts")
			Expect(preview.Timeout).To(BeNumerically(">", 0), "must disclose a bounded, non-zero timeout")
			Expect(preview.Timeout).To(BeNumerically("<=", 10*time.Minute))
		})
	})

	DescribeTable("truthfully discloses argv, on-disk effect, network, script policy, and timeout per package-manager kind (SA-280-012)",
		func(kind, wantExecutable string, wantArgs []string, wantLockfileBasename string, wantSuppressionSubstrings []string) {
			preview, err := codesignalcli.BuildSetupPreview(
				codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage},
				projectPackageManager(kind),
				"/tmp/example-root",
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(preview.Executable).To(Equal(wantExecutable))
			Expect(preview.Args).To(Equal(wantArgs))

			Expect(preview.ExpectedChanges).To(ContainSubstring("node_modules"), "every frozen row's actual on-disk mutation is node_modules, not the lockfile")
			Expect(preview.ExpectedChanges).NotTo(
				MatchRegexp(setupPreviewLockfileRewriteClaimPattern),
				"none of the frozen rows can rewrite a lockfile -- npm ci and pnpm/bun's --frozen-lockfile install all fail instead of writing one",
			)
			if wantLockfileBasename != "" {
				Expect(preview.ExpectedChanges).To(ContainSubstring(wantLockfileBasename), "must name the lockfile basename this argv reads and never writes")
			} else {
				// Bun recognizes two lockfile variants (bun.lock, bun.lockb) and
				// BuildSetupPreview is not told which this repository has --
				// the disclosure must not cite either specific basename.
				Expect(preview.ExpectedChanges).NotTo(Or(ContainSubstring("bun.lock"), ContainSubstring("bun.lockb")), "must not name a specific lockfile variant it cannot confirm exists")
			}

			Expect(preview.NetworkDisclosure).To(And(ContainSubstring("network"), ContainSubstring("registry")), "must truthfully disclose that this command may reach the package registry")
			for _, wantSuppression := range wantSuppressionSubstrings {
				Expect(preview.ScriptSuppressionPolicy).To(ContainSubstring(wantSuppression), "must truthfully disclose every flag this row's argv actually passes to suppress scripts/config hazards -- a shared, one-size-fits-all disclosure string would silently under-disclose a row like pnpm's that carries an extra flag")
			}
			Expect(preview.Timeout).To(Equal(codesignalcli.SetupPreviewTimeout), "must disclose the bounded timeout that will actually be enforced")
		},
		Entry("npm", "npm", "npm", []string{"ci", "--ignore-scripts"}, "package-lock.json", []string{"--ignore-scripts"}),
		Entry("pnpm", "pnpm", "pnpm", []string{"install", "--frozen-lockfile", "--ignore-scripts", "--ignore-pnpmfile"}, "pnpm-lock.yaml", []string{"--ignore-scripts", "--ignore-pnpmfile"}),
		Entry("bun", "bun", "bun", []string{"install", "--frozen-lockfile", "--ignore-scripts"}, "", []string{"--ignore-scripts"}),
	)

	When("checks.package_manager recorded a packageManager pin that differs from the version actually on PATH", func() {
		It("discloses that the pinned release is not what this command will run, naming both versions (AC-SET-2)", func() {
			preview, err := codesignalcli.BuildSetupPreview(
				codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage},
				codesignalcli.ReadinessCheck{State: codesignalcli.ReadinessPass, Kind: "npm", Version: "11.4.1", PinnedVersion: "11.2.0"},
				"/tmp/example-root",
			)
			Expect(err).NotTo(HaveOccurred())

			Expect(preview.PinDisclosure).To(ContainSubstring("11.2.0"), "the pin the manifest asked for must be named, not silently dropped")
			Expect(preview.PinDisclosure).To(ContainSubstring("11.4.1"), "the version that will actually run must be named beside it")
		})
	})

	When("checks.package_manager recorded no packageManager pin, or one matching the version on PATH", func() {
		It("discloses nothing about a pin, since there is no divergence for a customer to weigh", func() {
			unpinned, err := codesignalcli.BuildSetupPreview(
				codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage},
				codesignalcli.ReadinessCheck{State: codesignalcli.ReadinessPass, Kind: "npm", Version: "11.4.1"},
				"/tmp/example-root",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(unpinned.PinDisclosure).To(BeEmpty())

			agreeing, err := codesignalcli.BuildSetupPreview(
				codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage},
				codesignalcli.ReadinessCheck{State: codesignalcli.ReadinessPass, Kind: "npm", Version: "11.4.1", PinnedVersion: "11.4.1"},
				"/tmp/example-root",
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(agreeing.PinDisclosure).To(BeEmpty())
		})
	})

	When("the selected choice is not project-package", func() {
		It("fails closed rather than guessing a command", func() {
			_, err := codesignalcli.BuildSetupPreview(
				codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectMise},
				projectPackageManager("npm"),
				"/tmp/example-root",
			)
			Expect(err).To(HaveOccurred())
		})
	})

	When("packageManagerKind names a manager outside the frozen adapter matrix", func() {
		It("fails closed rather than guessing a command", func() {
			_, err := codesignalcli.BuildSetupPreview(
				codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage},
				projectPackageManager("yarn"),
				"/tmp/example-root",
			)
			Expect(err).To(HaveOccurred())
		})
	})
})

// setupPreviewLockfileRewriteClaimPattern matches ExpectedChanges wording
// that claims a lockfile may be rewritten, updated, or otherwise modified --
// a claim that is false for every frozen row (SA-280-012): npm ci and
// pnpm/bun's --frozen-lockfile install all fail instead of writing a
// lockfile. Deliberately not anchored to "never"/"leaves ... unchanged"
// phrasing, so a regression that reintroduces the false claim in either
// polarity ("may rewrite" or "never modifies") is caught.
const setupPreviewLockfileRewriteClaimPattern = `(?i)rewrite|update|modif\w+ (the )?(package-lock|pnpm-lock|bun\.lock)`
