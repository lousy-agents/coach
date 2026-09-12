package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

// writeStubSetupExecutable writes an executable script named `name` into a
// fresh temp directory that, on every invocation, records its working
// directory, its argv (one argument per line, so a shell that folded
// multiple tokens into one string would show up as a single logged line
// instead of several), and its entire environment (one "KEY=VALUE" line per
// variable, via the `env` builtin) into three log files inside that same
// directory, then exits 0 without doing anything else. Returns the
// directory.
func writeStubSetupExecutable(name string) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-stubsetup-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := fmt.Sprintf(
		"#!/bin/sh\nprintf '%%s' \"$PWD\" > %q\n: > %q\nfor a in \"$@\"; do printf '%%s\\n' \"$a\" >> %q; done\nenv > %q\nexit 0\n",
		filepath.Join(dir, name+".cwd"),
		filepath.Join(dir, name+".argv"),
		filepath.Join(dir, name+".argv"),
		filepath.Join(dir, name+".env"),
	)
	Expect(os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755)).To(Succeed())
	return dir
}

func stubSetupInvoked(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name+".argv"))
	return err == nil
}

// readStubSetupEnvKeys returns the set of environment-variable names the
// stub named `name` observed, parsed from its "KEY=VALUE" env dump.
func readStubSetupEnvKeys(dir, name string) []string {
	data, err := os.ReadFile(filepath.Join(dir, name+".env"))
	Expect(err).NotTo(HaveOccurred(), "expected the stub %s to have recorded its environment", name)
	var keys []string
	for _, line := range strings.Split(strings.TrimRight(string(data), "\n"), "\n") {
		if line == "" {
			continue
		}
		key, _, _ := strings.Cut(line, "=")
		keys = append(keys, key)
	}
	return keys
}

func readStubSetupCwd(dir, name string) string {
	data, err := os.ReadFile(filepath.Join(dir, name+".cwd"))
	Expect(err).NotTo(HaveOccurred(), "expected the stub %s to have recorded its working directory", name)
	return string(data)
}

func readStubSetupArgv(dir, name string) []string {
	data, err := os.ReadFile(filepath.Join(dir, name+".argv"))
	Expect(err).NotTo(HaveOccurred(), "expected the stub %s to have recorded its argv", name)
	trimmed := strings.TrimRight(string(data), "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}

// setupExecutionOnlyPath returns the current PATH with every npm/pnpm/bun/
// yarn directory removed, so a spec that prepends its own stub directory
// controls exactly which one of those names resolves.
func setupExecutionOnlyPath() string {
	return pathExcludingExecutables("npm", "pnpm", "bun", "yarn")
}

// writeOutlivingSetupExecutable writes an executable named `name` that
// models npm/pnpm/bun's real failure mode under a deadline: it forks a
// background descendant (detached via `&`, inheriting the same stdout/
// stderr pipe as the direct child) that outlives backgroundSeconds, then the
// direct process itself sleeps past any reasonable deadline. exec.
// CommandContext only ever signals the direct child, so with no WaitDelay
// the background descendant alone keeps the shared pipe open and cmd.Wait
// blocks for the entirety of backgroundSeconds regardless of the run's
// context deadline.
func writeOutlivingSetupExecutable(name string, backgroundSeconds int) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-stubsetup-outlive-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := fmt.Sprintf("#!/bin/sh\n(sleep %d) &\nsleep %d\n", backgroundSeconds, backgroundSeconds)
	Expect(os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755)).To(Succeed())
	return dir
}

// writeSucceedingSetupExecutableWithOutlivingDescendant writes an executable
// named `name` that models a real, successful install that happens to leave
// an orphaned background process behind (pnpm's store server, npm's
// update-notifier check, etc.): it forks a background descendant (detached
// via `&`, inheriting the same stdout/stderr pipe as the direct child) that
// outlives backgroundSeconds, then the direct process itself exits 0
// immediately -- unlike writeOutlivingSetupExecutable, the direct child never
// blocks. This is exec.ErrWaitDelay's real trigger: Go's os/exec docs record
// that the direct child can exit successfully while cmd.Wait still blocks
// because a descendant inherited its output pipe and kept it open.
func writeSucceedingSetupExecutableWithOutlivingDescendant(name string, backgroundSeconds int) string {
	dir, err := os.MkdirTemp("", "coach-acceptance-stubsetup-succeed-outlive-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)

	script := fmt.Sprintf("#!/bin/sh\n(sleep %d) &\nexit 0\n", backgroundSeconds)
	Expect(os.WriteFile(filepath.Join(dir, name), []byte(script), 0o755)).To(Succeed())
	return dir
}

func newSetupExecutionWorkDir() string {
	dir, err := os.MkdirTemp("", "coach-acceptance-setupexec-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, dir)
	return dir
}

var _ = Describe("codesignalcli.ExecuteSetup", func() {
	When("confirmed is false", func() {
		It("refuses without starting any subprocess (AC-SET-3)", func() {
			workDir := newSetupExecutionWorkDir()
			stubDir := writeStubSetupExecutable("npm")
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+setupExecutionOnlyPath())

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "npm", workDir)
			Expect(err).NotTo(HaveOccurred())

			result, execErr := codesignalcli.ExecuteSetup(context.Background(), preview, false)
			Expect(errors.Is(execErr, codesignalcli.ErrSetupExecutionNotConfirmed)).To(BeTrue())
			Expect(result).To(Equal(codesignalcli.SetupExecutionResult{}))
			Expect(stubSetupInvoked(stubDir, "npm")).To(BeFalse(), "no subprocess may start before a single explicit confirmation is supplied")
		})
	})

	When("confirmed is true", func() {
		It("executes exactly the previewed executable, args, and working directory -- nothing re-derived (AC-SET-3, AC-17)", func() {
			workDir := newSetupExecutionWorkDir()
			stubDir := writeStubSetupExecutable("npm")
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+setupExecutionOnlyPath())

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "npm", workDir)
			Expect(err).NotTo(HaveOccurred())

			result, execErr := codesignalcli.ExecuteSetup(context.Background(), preview, true)
			Expect(execErr).NotTo(HaveOccurred())
			Expect(result.Succeeded).To(BeTrue(), "output: %s", result.Output)
			Expect(result.ExitCode).To(Equal(0))

			Expect(result.Executable).To(Equal(preview.Executable))
			Expect(result.Args).To(Equal(preview.Args))
			Expect(result.WorkingDirectory).To(Equal(preview.WorkingDirectory))

			Expect(readStubSetupCwd(stubDir, "npm")).To(Equal(workDir))
			Expect(readStubSetupArgv(stubDir, "npm")).To(Equal(preview.Args))
		})
	})

	When("a hostile ambient environment variable is set", func() {
		It("confines the child to exactly PATH and HOME, dropping every ambient variable (SA-280-012)", func() {
			workDir := newSetupExecutionWorkDir()
			stubDir := writeStubSetupExecutable("npm")
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+setupExecutionOnlyPath())

			// Each of these would, if forwarded, re-enable or redirect the
			// very thing --ignore-scripts and the frozen argv are supposed to
			// guarantee: a re-enabled lifecycle-script setting, an injected
			// Node startup flag, and a registry override.
			GinkgoT().Setenv("npm_config_ignore_scripts", "false")
			GinkgoT().Setenv("NODE_OPTIONS", "--require ./evil.js")
			GinkgoT().Setenv("npm_config_registry", "http://127.0.0.1:9/attacker")

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "npm", workDir)
			Expect(err).NotTo(HaveOccurred())

			result, execErr := codesignalcli.ExecuteSetup(context.Background(), preview, true)
			Expect(execErr).NotTo(HaveOccurred())
			Expect(result.Succeeded).To(BeTrue(), "output: %s", result.Output)

			observedKeys := readStubSetupEnvKeys(stubDir, "npm")
			Expect(observedKeys).NotTo(ContainElement("npm_config_ignore_scripts"), "an ambient lifecycle-script override must never reach the child")
			Expect(observedKeys).NotTo(ContainElement("NODE_OPTIONS"), "an ambient Node startup-flag injection must never reach the child")
			Expect(observedKeys).NotTo(ContainElement("npm_config_registry"), "an ambient registry override must never reach the child")
			Expect(observedKeys).To(ContainElement("PATH"), "the child needs PATH to resolve npm and node")
			for _, key := range observedKeys {
				// PWD is synthesized by the /bin/sh stub itself (dash/bash both
				// export it unconditionally), not something ExecuteSetup passed
				// through -- it is not a confinement leak.
				Expect(key).To(Or(Equal("PATH"), Equal("HOME"), Equal("PWD")), "the child's environment must contain nothing beyond PATH, HOME, and the shell's own PWD, observed %q", key)
			}
		})
	})

	When("the preview's timeout has been altered from the frozen matrix's timeout", func() {
		It("refuses to execute a command whose disclosed timeout it did not itself verify (AC-14, AC-17)", func() {
			workDir := newSetupExecutionWorkDir()
			stubDir := writeStubSetupExecutable("npm")
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+setupExecutionOnlyPath())

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "npm", workDir)
			Expect(err).NotTo(HaveOccurred())
			tampered := preview
			tampered.Timeout = 1000 * time.Hour // a preview claiming 5 minutes must never actually run unbounded

			result, execErr := codesignalcli.ExecuteSetup(context.Background(), tampered, true)
			Expect(errors.Is(execErr, codesignalcli.ErrSetupExecutionUnverifiedCommand)).To(BeTrue())
			Expect(result).To(Equal(codesignalcli.SetupExecutionResult{}))
			Expect(stubSetupInvoked(stubDir, "npm")).To(BeFalse(), "a command whose disclosed timeout was altered must never execute")
		})
	})

	When("the working directory's name contains shell metacharacters", func() {
		It("passes it to the child as a single literal working directory, never shell-interpreted (AC-14)", func() {
			parent := newSetupExecutionWorkDir()

			// A real shell-based implementation building `cd <dir> && npm ci
			// ...` without perfect quoting would either fail to parse this as
			// one token (the install never runs) or actually execute the
			// injected commands. Either way it is observably different from
			// argv-literal execution, which must succeed with the directory
			// used exactly as-is.
			maliciousName := "repo dir; rm -rf . ; $(touch injected-a) `touch injected-b` #"
			workDir := filepath.Join(parent, maliciousName)
			Expect(os.Mkdir(workDir, 0o755)).To(Succeed())
			canary := filepath.Join(workDir, "canary.txt")
			Expect(os.WriteFile(canary, []byte("intact"), 0o644)).To(Succeed())

			stubDir := writeStubSetupExecutable("npm")
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+setupExecutionOnlyPath())

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "npm", workDir)
			Expect(err).NotTo(HaveOccurred())

			result, execErr := codesignalcli.ExecuteSetup(context.Background(), preview, true)
			Expect(execErr).NotTo(HaveOccurred())
			Expect(result.Succeeded).To(BeTrue(), "output: %s", result.Output)

			Expect(readStubSetupCwd(stubDir, "npm")).To(Equal(workDir), "the child's cwd must be the exact literal string, byte for byte -- proof no shell re-tokenized it")
			Expect(readStubSetupArgv(stubDir, "npm")).To(Equal([]string{"ci", "--ignore-scripts"}), "argv must be exactly the two frozen tokens, unaffected by the working directory's contents")

			_, injectedA := os.Stat(filepath.Join(parent, "injected-a"))
			Expect(os.IsNotExist(injectedA)).To(BeTrue(), "no command embedded in the directory name may have run")
			_, injectedB := os.Stat(filepath.Join(parent, "injected-b"))
			Expect(os.IsNotExist(injectedB)).To(BeTrue())
			data, readErr := os.ReadFile(canary)
			Expect(readErr).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("intact"), "nothing inside the working directory may have been altered")
		})
	})

	When("the preview names a manager outside the frozen adapter matrix", func() {
		It("refuses to execute rather than falling back to a best-effort command (AC-4, AC-14)", func() {
			workDir := newSetupExecutionWorkDir()
			stubDir := writeStubSetupExecutable("yarn")
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+setupExecutionOnlyPath())

			preview := codesignalcli.SetupPreview{
				Executable:       "yarn",
				Args:             []string{"install", "--ignore-scripts"},
				WorkingDirectory: workDir,
				Timeout:          codesignalcli.SetupPreviewTimeout,
			}

			result, execErr := codesignalcli.ExecuteSetup(context.Background(), preview, true)
			Expect(errors.Is(execErr, codesignalcli.ErrSetupExecutionUnverifiedCommand)).To(BeTrue())
			Expect(result).To(Equal(codesignalcli.SetupExecutionResult{}))
			Expect(stubSetupInvoked(stubDir, "yarn")).To(BeFalse(), "an adapter outside the frozen matrix must never reach a subprocess")
		})
	})

	When("the preview's args have been altered from the frozen npm template", func() {
		It("refuses to execute a command it did not itself verify against the matrix (AC-14, AC-17)", func() {
			workDir := newSetupExecutionWorkDir()
			stubDir := writeStubSetupExecutable("npm")
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+setupExecutionOnlyPath())

			tampered := codesignalcli.SetupPreview{
				Executable:       "npm",
				Args:             []string{"ci"}, // --ignore-scripts dropped
				WorkingDirectory: workDir,
				Timeout:          codesignalcli.SetupPreviewTimeout,
			}

			result, execErr := codesignalcli.ExecuteSetup(context.Background(), tampered, true)
			Expect(errors.Is(execErr, codesignalcli.ErrSetupExecutionUnverifiedCommand)).To(BeTrue())
			Expect(result).To(Equal(codesignalcli.SetupExecutionResult{}))
			Expect(stubSetupInvoked(stubDir, "npm")).To(BeFalse(), "a command that dropped --ignore-scripts must never execute")
		})
	})

	When("the command outlives its deadline and leaves a background descendant holding the output pipe open", func() {
		It("still returns within a bounded time, reporting TimedOut and ExitCode -1 (AC-SET-2)", func() {
			workDir := newSetupExecutionWorkDir()
			// The background descendant outlives the assertion bound below by
			// a wide margin, so a pre-fix run (no WaitDelay) would still be
			// blocked in cmd.Wait when the assertion's deadline is checked --
			// this is what makes the bound below a genuine proof, not a race.
			stubDir := writeOutlivingSetupExecutable("npm", 20)
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+setupExecutionOnlyPath())

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "npm", workDir)
			Expect(err).NotTo(HaveOccurred())

			// preview.Timeout stays the frozen SetupPreviewTimeout (5 minutes,
			// required by ExecuteSetup's matrix verification); the short
			// deadline that actually bounds this run comes from ctx, whose
			// earlier deadline wins once ExecuteSetup derives runCtx from it.
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
			defer cancel()

			start := time.Now()
			result, execErr := codesignalcli.ExecuteSetup(ctx, preview, true)
			elapsed := time.Since(start)

			Expect(execErr).NotTo(HaveOccurred())
			Expect(elapsed).To(BeNumerically("<", 8*time.Second), "ExecuteSetup must force-close the output pipe via WaitDelay rather than block on a surviving background descendant")
			Expect(result.TimedOut).To(BeTrue())
			Expect(result.ExitCode).To(Equal(-1))
			Expect(result.Succeeded).To(BeFalse())
		})
	})

	When("the command exits successfully on its own but a background descendant keeps the output pipe open past WaitDelay", func() {
		It("still reports the real exit-0 outcome, recovering it from cmd.ProcessState rather than misreporting exec.ErrWaitDelay as a failure", func() {
			workDir := newSetupExecutionWorkDir()
			// The direct child exits 0 immediately; only its background
			// descendant survives, and only long enough to still be holding
			// the pipe open once setupExecutionWaitDelay elapses -- so
			// cmd.Wait returns exec.ErrWaitDelay even though the run
			// genuinely succeeded. Nothing here cancels ctx, so
			// runCtx.Err() is nil: this is not the timeout path above.
			stubDir := writeSucceedingSetupExecutableWithOutlivingDescendant("npm", 20)
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+setupExecutionOnlyPath())

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "npm", workDir)
			Expect(err).NotTo(HaveOccurred())

			start := time.Now()
			result, execErr := codesignalcli.ExecuteSetup(context.Background(), preview, true)
			elapsed := time.Since(start)

			Expect(execErr).NotTo(HaveOccurred())
			Expect(elapsed).To(BeNumerically("<", 8*time.Second), "must not block on the surviving background descendant, only on setupExecutionWaitDelay")
			Expect(result.TimedOut).To(BeFalse(), "the run was never cancelled by ctx or the preview timeout -- only its output pipe stayed open")
			Expect(result.Succeeded).To(BeTrue(), "the direct child exited 0; a still-open pipe held by an unrelated background descendant must not turn a successful install into a reported failure. output: %s", result.Output)
			Expect(result.ExitCode).To(Equal(0))
		})
	})
})

// requireRealPackageManager skips the current spec, rather than failing it,
// when name is not installed on this machine: the sentinel-script proofs
// below need the real package manager binary to prove its own
// --ignore-scripts contract, not a stand-in for it.
func requireRealPackageManager(name string) {
	if _, err := exec.LookPath(name); err != nil {
		Skip(fmt.Sprintf("%s not found on PATH; skipping the real-execution sentinel-script proof for %s (%s)", name, name, err))
	}
}

// writeSentinelDependencyFixture writes a local ("file:") dependency at
// vendor/sentinel-pkg under repoDir whose postinstall script writes markerPath
// if a package manager ever actually runs it.
func writeSentinelDependencyFixture(repoDir string) (markerPath string) {
	vendorDir := filepath.Join(repoDir, "vendor", "sentinel-pkg")
	Expect(os.MkdirAll(vendorDir, 0o755)).To(Succeed())
	markerPath = filepath.Join(repoDir, "postinstall-ran.marker")

	postinstall := fmt.Sprintf(`node -e "require('fs').writeFileSync('%s', 'ran')"`, markerPath)
	doc := map[string]any{
		"name":    "sentinel-pkg",
		"version": "1.0.0",
		"scripts": map[string]string{"postinstall": postinstall},
	}
	data, err := json.Marshal(doc)
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(filepath.Join(vendorDir, "package.json"), data, 0o644)).To(Succeed())
	return markerPath
}

// writeSentinelRootPackageJSON writes repoDir's own package.json depending on
// the vendor/sentinel-pkg fixture. pnpm and bun both refuse to run a
// dependency's lifecycle scripts by default regardless of --ignore-scripts'
// absence (their own, separate script-allowlist security feature) unless the
// dependency is explicitly trusted -- allowPnpmBuild/trustBunDependency opt
// sentinel-pkg into that allowlist so --ignore-scripts is the only thing left
// deciding whether the postinstall runs, which is the property under test.
func writeSentinelRootPackageJSON(repoDir string, allowPnpmBuild, trustBunDependency bool) {
	doc := map[string]any{
		"name":         "example",
		"version":      "1.0.0",
		"dependencies": map[string]string{"sentinel-pkg": "file:vendor/sentinel-pkg"},
	}
	if allowPnpmBuild {
		doc["pnpm"] = map[string]any{"onlyBuiltDependencies": []string{"sentinel-pkg"}}
	}
	if trustBunDependency {
		doc["trustedDependencies"] = []string{"sentinel-pkg"}
	}
	data, err := json.Marshal(doc)
	Expect(err).NotTo(HaveOccurred())
	Expect(os.WriteFile(filepath.Join(repoDir, "package.json"), data, 0o644)).To(Succeed())
}

func requireSentinelMarkerAppeared(markerPath string) {
	_, statErr := os.Stat(markerPath)
	Expect(statErr).NotTo(HaveOccurred(), "expected the sentinel postinstall script to have run and created %s -- otherwise this fixture cannot prove suppression actually works", markerPath)
}

func requireSentinelMarkerAbsent(markerPath, becauseOf string) {
	_, statErr := os.Stat(markerPath)
	Expect(os.IsNotExist(statErr)).To(BeTrue(), "%s must suppress the dependency's postinstall script", becauseOf)
}

func resetSentinelInstall(repoDir, markerPath string) {
	Expect(os.RemoveAll(filepath.Join(repoDir, "node_modules"))).To(Succeed())
	Expect(os.Remove(markerPath)).To(Succeed())
}

var _ = Describe("codesignalcli.ExecuteSetup script suppression (AC-4)", func() {
	When("a dependency's own postinstall script would create a marker file if it ran", func() {
		It("suppresses it under npm's --ignore-scripts adapter", func() {
			requireRealPackageManager("npm")

			repoDir := newSetupExecutionWorkDir()
			markerPath := writeSentinelDependencyFixture(repoDir)
			writeSentinelRootPackageJSON(repoDir, false, false)

			generate := exec.Command("npm", "install", "--package-lock-only", "--ignore-scripts")
			generate.Dir = repoDir
			out, genErr := generate.CombinedOutput()
			Expect(genErr).NotTo(HaveOccurred(), "generating the npm lockfile fixture: %s", out)
			Expect(filepath.Join(repoDir, "package-lock.json")).To(BeAnExistingFile())

			// Negative control: prove the fixture actually would create the
			// marker without --ignore-scripts, so the suppression assertion
			// below cannot be a false green caused by an inert fixture.
			control := exec.Command("npm", "ci")
			control.Dir = repoDir
			out, ctrlErr := control.CombinedOutput()
			Expect(ctrlErr).NotTo(HaveOccurred(), "control npm ci: %s", out)
			requireSentinelMarkerAppeared(markerPath)
			resetSentinelInstall(repoDir, markerPath)

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "npm", repoDir)
			Expect(err).NotTo(HaveOccurred())

			result, execErr := codesignalcli.ExecuteSetup(context.Background(), preview, true)
			Expect(execErr).NotTo(HaveOccurred())
			Expect(result.Succeeded).To(BeTrue(), "output: %s", result.Output)

			Expect(filepath.Join(repoDir, "node_modules", "sentinel-pkg", "package.json")).To(BeAnExistingFile(), "the dependency must actually have been installed")
			requireSentinelMarkerAbsent(markerPath, "npm ci --ignore-scripts")
		})

		It("suppresses it under pnpm's --ignore-scripts adapter", func() {
			requireRealPackageManager("pnpm")

			repoDir := newSetupExecutionWorkDir()
			markerPath := writeSentinelDependencyFixture(repoDir)
			writeSentinelRootPackageJSON(repoDir, true, false)

			generate := exec.Command("pnpm", "install", "--lockfile-only", "--ignore-scripts")
			generate.Dir = repoDir
			out, genErr := generate.CombinedOutput()
			Expect(genErr).NotTo(HaveOccurred(), "generating the pnpm lockfile fixture: %s", out)
			Expect(filepath.Join(repoDir, "pnpm-lock.yaml")).To(BeAnExistingFile())

			control := exec.Command("pnpm", "install", "--frozen-lockfile")
			control.Dir = repoDir
			out, ctrlErr := control.CombinedOutput()
			Expect(ctrlErr).NotTo(HaveOccurred(), "control pnpm install: %s", out)
			requireSentinelMarkerAppeared(markerPath)
			resetSentinelInstall(repoDir, markerPath)

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "pnpm", repoDir)
			Expect(err).NotTo(HaveOccurred())

			result, execErr := codesignalcli.ExecuteSetup(context.Background(), preview, true)
			Expect(execErr).NotTo(HaveOccurred())
			Expect(result.Succeeded).To(BeTrue(), "output: %s", result.Output)

			Expect(filepath.Join(repoDir, "node_modules", "sentinel-pkg", "package.json")).To(BeAnExistingFile(), "the dependency must actually have been installed")
			requireSentinelMarkerAbsent(markerPath, "pnpm install --frozen-lockfile --ignore-scripts")
		})

		It("suppresses it under bun's --ignore-scripts adapter", func() {
			requireRealPackageManager("bun")

			repoDir := newSetupExecutionWorkDir()
			markerPath := writeSentinelDependencyFixture(repoDir)
			writeSentinelRootPackageJSON(repoDir, false, true)

			generate := exec.Command("bun", "install", "--lockfile-only", "--ignore-scripts")
			generate.Dir = repoDir
			out, genErr := generate.CombinedOutput()
			Expect(genErr).NotTo(HaveOccurred(), "generating the bun lockfile fixture: %s", out)

			control := exec.Command("bun", "install", "--frozen-lockfile")
			control.Dir = repoDir
			out, ctrlErr := control.CombinedOutput()
			Expect(ctrlErr).NotTo(HaveOccurred(), "control bun install: %s", out)
			requireSentinelMarkerAppeared(markerPath)
			resetSentinelInstall(repoDir, markerPath)

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "bun", repoDir)
			Expect(err).NotTo(HaveOccurred())

			result, execErr := codesignalcli.ExecuteSetup(context.Background(), preview, true)
			Expect(execErr).NotTo(HaveOccurred())
			Expect(result.Succeeded).To(BeTrue(), "output: %s", result.Output)

			Expect(filepath.Join(repoDir, "node_modules", "sentinel-pkg", "package.json")).To(BeAnExistingFile(), "the dependency must actually have been installed")
			requireSentinelMarkerAbsent(markerPath, "bun install --frozen-lockfile --ignore-scripts")
		})
	})

	// pnpm loads and executes a committed .pnpmfile.cjs during install --
	// including its module top level -- independently of --ignore-scripts;
	// --ignore-pnpmfile is the separate opt-out. Bun's analogous vector, a
	// bunfig.toml "preload" entry, was checked and does not fire on
	// `bun install` (only on `bun run`/the bun runtime), so it needs no
	// equivalent spec here.
	When("a committed .pnpmfile.cjs would create a marker file if pnpm ever loaded it", func() {
		It("suppresses it under pnpm's --ignore-pnpmfile adapter (AC-4)", func() {
			requireRealPackageManager("pnpm")

			repoDir := newSetupExecutionWorkDir()
			Expect(os.WriteFile(filepath.Join(repoDir, "package.json"), []byte(`{"name":"example","version":"1.0.0"}`+"\n"), 0o644)).To(Succeed())
			markerPath := writePnpmfileHazardFixture(repoDir)

			generate := exec.Command("pnpm", "install", "--lockfile-only", "--ignore-scripts", "--ignore-pnpmfile")
			generate.Dir = repoDir
			out, genErr := generate.CombinedOutput()
			Expect(genErr).NotTo(HaveOccurred(), "generating the pnpm lockfile fixture: %s", out)
			_, markerAfterGenerate := os.Stat(markerPath)
			Expect(os.IsNotExist(markerAfterGenerate)).To(BeTrue(), "generating the fixture lockfile must itself use --ignore-pnpmfile, or the negative control below would prove nothing")

			// Negative control: --ignore-scripts alone does not stop pnpm from
			// loading and running a committed .pnpmfile.cjs.
			control := exec.Command("pnpm", "install", "--frozen-lockfile", "--ignore-scripts")
			control.Dir = repoDir
			out, ctrlErr := control.CombinedOutput()
			Expect(ctrlErr).NotTo(HaveOccurred(), "control pnpm install: %s", out)
			requireSentinelMarkerAppeared(markerPath)
			resetSentinelInstall(repoDir, markerPath)

			preview, err := codesignalcli.BuildSetupPreview(codesignalcli.SetupChoice{Kind: codesignalcli.SetupChoiceProjectPackage}, "pnpm", repoDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(preview.Args).To(ContainElement("--ignore-pnpmfile"), "the frozen pnpm template must suppress .pnpmfile.cjs execution")

			result, execErr := codesignalcli.ExecuteSetup(context.Background(), preview, true)
			Expect(execErr).NotTo(HaveOccurred())
			Expect(result.Succeeded).To(BeTrue(), "output: %s", result.Output)

			requireSentinelMarkerAbsent(markerPath, "pnpm install --frozen-lockfile --ignore-scripts --ignore-pnpmfile")
		})
	})
})

// writePnpmfileHazardFixture writes a .pnpmfile.cjs at repoDir whose module
// top level -- code pnpm runs merely by loading the file, before any hook is
// even called -- writes markerPath if pnpm ever actually loads it.
func writePnpmfileHazardFixture(repoDir string) (markerPath string) {
	markerPath = filepath.Join(repoDir, "pnpmfile-ran.marker")
	doc := fmt.Sprintf("require('fs').writeFileSync(%q, 'ran');\n", markerPath)
	Expect(os.WriteFile(filepath.Join(repoDir, ".pnpmfile.cjs"), []byte(doc), 0o644)).To(Succeed())
	return markerPath
}
