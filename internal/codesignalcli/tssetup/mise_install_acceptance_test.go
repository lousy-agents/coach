package tssetup

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// lifecycleSentinelFile is the file coach-lifecycle-sentinel's pre/postinstall
// scripts write if ever executed (see the fixture's own package.json).
const lifecycleSentinelFile = "LIFECYCLE_SCRIPT_RAN"

const (
	lifecycleSentinelPackageName = "coach-lifecycle-sentinel"
	lifecycleSentinelVersion     = "1.2.3"
	lifecycleSentinelToolSpec    = "npm:" + lifecycleSentinelPackageName + "@" + lifecycleSentinelVersion
)

var _ = Describe("mise npm-backend install command construction: lifecycle-script suppression (coach#328 Task 4, AC-4/AC-15)", func() {
	When("mise's default (aube) backend installs the lifecycle-sentinel fixture from a loopback npm registry", func() {
		It("never runs the fixture's lifecycle scripts, checked at the tarball's landing site under MISE_DATA_DIR", neverRunsFixturesLifecycleScriptsCheckedTarballs)
	})

	When("mise's npm.shell_out setting forces it to shell out to a real npm for the install", func() {
		It("still never runs the fixture's lifecycle scripts under that real npm invocation", stillNeverRunsFixturesLifecycleScriptsUnder)
	})
})

var _ = Describe("mise npm-backend install command construction: insulated working directory (coach#328 Task 4, AC-16)", func() {
	When("the analyzed repository's mise.toml carries a trusted env exec template that would write a sentinel", func() {
		It("never executes it, because the real install runs from a private neutral working directory that never discovers the repository's config", neverExecutesBecauseRealInstallRunsFrom)
	})
})

var _ = Describe("mise npm-backend install command construction: post-launch failure classification (coach#328 Task 4, AC-SET-7)", func() {
	When("the install subprocess starts but exceeds the stdout budget before exiting", func() {
		It("reports attempted=true, observed=false -- disk state may already have changed even though the outcome could not be read back", func() {
			stubDir := writeStdoutOverflowStubMise()
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

			attempt := runMiseInstallInsulated(context.Background(), "npm:file:does-not-matter")
			attempted, observed, exitErr := attempt.attempted, attempt.observed, attempt.exitErr
			Expect(attempted).To(BeTrue(), "the subprocess actually started (cmd.Start() succeeded); the AC-SET-7 disk-may-have-changed guarantee must hold even though the outcome could not be observed")
			Expect(observed).To(BeFalse(), "the stdout budget was exceeded, so the outcome could not be read back")
			Expect(exitErr).NotTo(HaveOccurred())
		})
	})
})

var _ = Describe("mise npm-backend install command construction: trust gate and compiler verification (coach#328 Task 4, AC-4)", func() {
	var (
		originalProbeVersion func(context.Context) (string, bool)
		originalGlobalHazard func(context.Context) bool
		originalLocate       func(context.Context, string) (string, bool)
	)

	BeforeEach(func() {
		originalProbeVersion = tstoolchain.ProbeMiseToolVersion
		originalGlobalHazard = tstoolchain.ProbeMiseGlobalConfigHazard
		originalLocate = tstoolchain.LocateMiseTypescriptInstall
		tstoolchain.ProbeMiseToolVersion = func(context.Context) (string, bool) { return "2026.9.5 linux-x64 (2026-09-10)", true }
		tstoolchain.ProbeMiseGlobalConfigHazard = func(context.Context) bool { return false }
	})

	AfterEach(func() {
		tstoolchain.ProbeMiseToolVersion = originalProbeVersion
		tstoolchain.ProbeMiseGlobalConfigHazard = originalGlobalHazard
		tstoolchain.LocateMiseTypescriptInstall = originalLocate
	})

	Context("refuses before ever attempting install", func() {
		When("the project mise.toml carries an execution hazard ([hooks])", func() {
			It("refuses with package_manager_config_unverifiable before ever attempting mise install", func() {
				repo := GinkgoT().TempDir()
				Expect(os.WriteFile(filepath.Join(repo, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n[hooks]\npostinstall = \"echo pwned\"\n"), 0o644)).To(Succeed())

				result := installMiseTypescriptProject(context.Background(), repo, "7.0.2")
				Expect(result.Trusted).To(BeFalse(), "a hazardous project mise.toml must refuse before ever running mise: %+v", result)
				Expect(result.Attempted).To(BeFalse(), "an untrusted scope must never attempt mise install: %+v", result)
				Expect(result.Code).To(Equal(projectreadiness.GapPackageManagerConfigUnverifiable))
			})
		})

		When("the trusted scope's private working directory cannot be created (coach#328 Task 5 integration repair, Finding 5)", func() {
			It("reports package_manager_config_unverifiable rather than a bare, codeless could-not-start failure", func() {

				repo := GinkgoT().TempDir()
				Expect(os.WriteFile(filepath.Join(repo, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"7.0.2\"\n"), 0o644)).To(Succeed())
				nonexistentTemp := filepath.Join(GinkgoT().TempDir(), "does-not-exist")
				GinkgoT().Setenv("TMPDIR", nonexistentTemp)

				result := installMiseTypescriptProject(context.Background(), repo, "7.0.2")
				Expect(result.Trusted).To(BeTrue(), "the trust gate itself never touches the working directory: %+v", result)
				Expect(result.Attempted).To(BeFalse(), "the install subprocess must never start when insulation could not be established: %+v", result)
				Expect(result.Observed).To(BeFalse(), "%+v", result)
				Expect(result.Code).To(Equal(projectreadiness.GapPackageManagerConfigUnverifiable), "insulation failing before any subprocess starts must be distinguishable from mise simply being absent from PATH: %+v", result)
			})
		})
	})

	Context("classifies the install subprocess's outcome", func() {
		When("the project mise scope is trusted and mise reports a successful install", func() {
			It("verifies the installed compiler via classifyCompilerCandidate and reports it eligible", func() {
				stubDir := writeNoOpStubMise()
				GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

				installDir := GinkgoT().TempDir()
				writeFakeInstalledTypescript(installDir, "7.0.2")
				tstoolchain.LocateMiseTypescriptInstall = func(context.Context, string) (string, bool) {
					return filepath.Join(installDir, "node_modules", "typescript"), true
				}

				repo := GinkgoT().TempDir()
				Expect(os.WriteFile(filepath.Join(repo, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"7.0.2\"\n"), 0o644)).To(Succeed())

				result := installMiseTypescriptProject(context.Background(), repo, "7.0.2")
				Expect(result.Trusted).To(BeTrue(), "%+v", result)
				Expect(result.Attempted).To(BeTrue(), "%+v", result)
				Expect(result.Succeeded).To(BeTrue(), "expected the freshly-installed compiler to classify as eligible: %+v", result)
				Expect(result.Class).To(Equal(tstoolchain.ClassEligible))
				Expect(result.Version).To(Equal("7.0.2"))
				Expect(result.Origin).To(Equal(tstoolchain.OriginMiseProject))
			})
		})

		When("the global mise scope is trusted and mise reports a successful install", func() {
			It("verifies the installed compiler via classifyCompilerCandidate and reports it eligible", func() {
				stubDir := writeNoOpStubMise()
				GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

				installDir := GinkgoT().TempDir()
				writeFakeInstalledTypescript(installDir, "7.0.2")
				tstoolchain.LocateMiseTypescriptInstall = func(context.Context, string) (string, bool) {
					return filepath.Join(installDir, "node_modules", "typescript"), true
				}

				result := installMiseTypescriptGlobal(context.Background(), "7.0.2")
				Expect(result.Trusted).To(BeTrue(), "%+v", result)
				Expect(result.Attempted).To(BeTrue(), "%+v", result)
				Expect(result.Succeeded).To(BeTrue(), "expected the freshly-installed compiler to classify as eligible: %+v", result)
				Expect(result.Origin).To(Equal(tstoolchain.OriginMiseGlobal))
			})
		})

		When("the project mise scope is trusted but the install subprocess exits non-zero", func() {
			It("reports Attempted=true, Succeeded=false, and no gap Code -- a just-run-but-failed install has no frozen gap code of its own", func() {
				stubDir := writeNonZeroExitStubMise()
				GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

				repo := GinkgoT().TempDir()
				Expect(os.WriteFile(filepath.Join(repo, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"7.0.2\"\n"), 0o644)).To(Succeed())

				result := installMiseTypescriptProject(context.Background(), repo, "7.0.2")
				Expect(result.Trusted).To(BeTrue(), "%+v", result)
				Expect(result.Attempted).To(BeTrue(), "expected the subprocess to have actually started: %+v", result)
				Expect(result.Observed).To(BeTrue(), "a non-zero exit is still an observed outcome -- only a timeout or output-budget overflow leaves Observed false: %+v", result)
				Expect(result.Succeeded).To(BeFalse(), "%+v", result)
				Expect(result.Code).To(BeEmpty(), "%+v", result)
				Expect(result.Class).To(BeEmpty(), "a non-zero exit must never reach classifyCompilerCandidate, so Class stays empty -- this is what distinguishes an actual install failure from a verification-only failure (coach#328 Task 5 integration repair, Finding 2): %+v", result)
			})
		})

		When("the project mise scope is trusted, mise install exits 0, but the freshly-installed compiler is not eligible (coach#328 Task 5 integration repair, Finding 2)", func() {
			It("reports Observed=true and a non-empty Class, distinguishing a verification-only failure from an actual install failure", func() {
				stubDir := writeNoOpStubMise()
				GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

				tstoolchain.LocateMiseTypescriptInstall = func(context.Context, string) (string, bool) {
					return "", false
				}

				repo := GinkgoT().TempDir()
				Expect(os.WriteFile(filepath.Join(repo, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"7.0.2\"\n"), 0o644)).To(Succeed())

				result := installMiseTypescriptProject(context.Background(), repo, "7.0.2")
				Expect(result.Trusted).To(BeTrue(), "%+v", result)
				Expect(result.Attempted).To(BeTrue(), "expected the subprocess to have actually started: %+v", result)
				Expect(result.Observed).To(BeTrue(), "the stub mise exits 0 immediately, so the outcome must be observed: %+v", result)
				Expect(result.Succeeded).To(BeFalse(), "%+v", result)
				Expect(result.Class).NotTo(BeEmpty(), "classifyCompilerCandidate must have run and produced a class, proving this is a verification failure rather than a subprocess failure: %+v", result)
			})
		})
	})
})

var _ = Describe("mise npm-backend install command construction: real end-to-end native-package classification (coach#328 Task 5 integration repair, Finding 1)", func() {
	When("mise's default npm backend genuinely installs the frozen row's TypeScript version, with no fabricated filesystem layout standing in for it", func() {
		It("classifies the freshly-installed compiler as eligible, not merely as an install that exited zero", classifiesFreshlyInstalledCompilerEligibleNotMerely)
	})
})

func neverRunsFixturesLifecycleScriptsCheckedTarballs() {
	if _, err := exec.LookPath("mise"); err != nil {
		Skip(fmt.Sprintf("mise not found on PATH; skipping the real mise install lifecycle-suppression spec (%s)", err))
	}
	if _, err := exec.LookPath("npm"); err != nil {
		Skip(fmt.Sprintf("npm not found on PATH; skipping the real mise install lifecycle-suppression spec (%s)", err))
	}
	dataDir, _ := freshMiseInstallEnv()
	writeHomeNpmrcRegistry(startLifecycleSentinelRegistry().URL)
	assertLifecycleSentinelFixtureIsLive(GinkgoT().TempDir())

	attempt := runMiseInstallInsulated(context.Background(), lifecycleSentinelToolSpec)
	attempted, observed, exitErr := attempt.attempted, attempt.observed, attempt.exitErr
	Expect(attempted).To(BeTrue(), "expected the install subprocess to actually start (mise confined and reachable)")
	Expect(observed).To(BeTrue(), "expected the install subprocess's outcome to be observable")
	Expect(exitErr).NotTo(HaveOccurred(), "mise install of the local fixture package must itself succeed")

	Expect(anyFileNamed(dataDir, lifecycleSentinelFile)).To(BeFalse(), "the fixture's pre/postinstall script must never run under mise's npm backend suppression, checked at the tarball's landing site under MISE_DATA_DIR")
}

func stillNeverRunsFixturesLifecycleScriptsUnder() {
	if _, err := exec.LookPath("mise"); err != nil {
		Skip(fmt.Sprintf("mise not found on PATH; skipping the mise npm.shell_out=true lifecycle-suppression spec (%s)", err))
	}
	if _, err := exec.LookPath("npm"); err != nil {
		Skip(fmt.Sprintf("npm not found on PATH; skipping the mise npm.shell_out=true lifecycle-suppression spec (%s)", err))
	}
	dataDir, configDir := freshMiseInstallEnv()
	writeHomeNpmrcRegistry(startLifecycleSentinelRegistry().URL)
	assertLifecycleSentinelFixtureIsLive(GinkgoT().TempDir())

	Expect(os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("[settings]\nnpm.shell_out = true\n"), 0o644)).To(Succeed())

	attempt := runMiseInstallInsulated(context.Background(), lifecycleSentinelToolSpec)
	attempted, observed, exitErr := attempt.attempted, attempt.observed, attempt.exitErr
	Expect(attempted).To(BeTrue(), "expected the install subprocess to actually start (mise confined and reachable)")
	Expect(observed).To(BeTrue(), "expected the install subprocess's outcome to be observable")
	Expect(exitErr).NotTo(HaveOccurred(), "mise install of the local fixture package must itself succeed under the shell_out=true backend")

	Expect(npmGlobalStyleInstallPresent(dataDir, lifecycleSentinelPackageName)).To(BeTrue(), "expected the npm.shell_out=true backend's real `npm install -g` layout under MISE_DATA_DIR, proving this spec actually reached that branch rather than silently falling back to the default backend")

	Expect(anyFileNamed(dataDir, lifecycleSentinelFile)).To(BeFalse(), "the fixture's pre/postinstall script must never run under the real npm invocation mise's shell_out=true backend shells out to, checked at the tarball's landing site under MISE_DATA_DIR")
}

func neverExecutesBecauseRealInstallRunsFrom() {
	if _, err := exec.LookPath("mise"); err != nil {
		Skip(fmt.Sprintf("mise not found on PATH; skipping the real mise insulated-working-directory spec (%s)", err))
	}
	freshMiseInstallEnv()
	writeHomeNpmrcRegistry(startLifecycleSentinelRegistry().URL)

	repo := GinkgoT().TempDir()
	sentinel := filepath.Join(repo, "mise-config-side-effect")
	decoy := fmt.Sprintf("[env]\nSIDE_EFFECT = \"{{ exec(command='touch %s') }}\"\n", sentinel)
	Expect(os.WriteFile(filepath.Join(repo, "mise.toml"), []byte(decoy), 0o644)).To(Succeed())

	Expect(tstoolchain.HasMiseConfigHazard(decoy)).To(BeFalse(), "sanity: this decoy construct must not be one hasMiseConfigHazard already refuses on, or this spec would not isolate AC-16's insulation guarantee")

	trustCmd := exec.Command("mise", "trust")
	trustCmd.Dir = repo
	trustOut, trustErr := trustCmd.CombinedOutput()
	Expect(trustErr).NotTo(HaveOccurred(), "mise trust: %s", trustOut)

	controlCmd := exec.Command("mise", "install", lifecycleSentinelToolSpec)
	controlCmd.Dir = repo
	controlOut, controlErr := controlCmd.CombinedOutput()
	Expect(controlErr).NotTo(HaveOccurred(), "mise install (positive control, cwd=repo): %s", controlOut)
	Expect(sentinel).To(BeAnExistingFile(), "expected the repository's mise.toml env exec template to fire for a real mise invocation whose working directory is the repository itself")
	Expect(os.Remove(sentinel)).To(Succeed())

	attempt := runMiseInstallInsulated(context.Background(), lifecycleSentinelToolSpec)
	attempted, observed, exitErr := attempt.attempted, attempt.observed, attempt.exitErr
	Expect(attempted).To(BeTrue())
	Expect(observed).To(BeTrue())
	Expect(exitErr).NotTo(HaveOccurred())

	_, statErr := os.Stat(sentinel)
	Expect(os.IsNotExist(statErr)).To(BeTrue(), "the repository's mise.toml env exec template must never execute during the insulated install")
}

func classifiesFreshlyInstalledCompilerEligibleNotMerely() {
	if _, err := exec.LookPath("mise"); err != nil {
		Skip(fmt.Sprintf("mise not found on PATH; skipping the real end-to-end mise install classification spec (%s)", err))
	}
	freshMiseInstallEnv()

	result := installMiseTypescriptGlobal(context.Background(), "7.0.2")
	Expect(result.Trusted).To(BeTrue(), "expected the fresh, hazard-free global mise scope to be trusted: %+v", result)
	Expect(result.Attempted).To(BeTrue(), "expected a real `mise install` subprocess to actually run: %+v", result)
	Expect(result.Succeeded).To(BeTrue(), "a real `mise install npm:typescript@7.0.2` must classify as compilerClassEligible, not merely exit zero -- coach#328 Finding 1: mise's own default npm backend does not hoist the platform-native optionalDependency package to the top-level location classifyCompilerCandidate expects unless something completes that hoisting: %+v", result)
	Expect(result.Class).To(Equal(tstoolchain.ClassEligible), "%+v", result)
	Expect(result.NativePath).NotTo(BeEmpty(), "%+v", result)
}
