package codesignalcli

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// This package's Ginkgo specs all run under the single TestProjectTextAcceptance
// entrypoint in project_acceptance_test.go (see project_snapshot_acceptance_test.go's
// comment for why this file defines no Test*Acceptance of its own).

// lifecycleSentinelFile is the file coach-lifecycle-sentinel's pre/postinstall
// scripts write if ever executed (see the fixture's own package.json).
const lifecycleSentinelFile = "LIFECYCLE_SCRIPT_RAN"

// lifecycleSentinelFixtureDir is the checked-in local npm package
// (cmd/coach/testdata/mise/lifecycle-sentinel/lifecycle-sentinel@1.2.3)
// whose pre/postinstall scripts write lifecycleSentinelFile if ever
// executed. Its directory name embeds "@1.2.3" -- verified empirically
// against mise 2026.9.5: mise's npm:file: backend (both the default aube
// backend and the npm.shell_out=true real-npm backend) re-derives its
// internal "specifier" from the directory name, so a toolSpec of
// "npm:file:<this-dir>" only resolves correctly when the directory's own
// name already carries the exact version suffix mise expects.
func lifecycleSentinelFixtureDir() string {
	_, thisFile, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue(), "runtime.Caller(0) failed")
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "cmd", "coach", "testdata", "mise", "lifecycle-sentinel", "lifecycle-sentinel@1.2.3")
}

// copyFileTree copies every file under src into dst, preserving relative
// paths, creating directories as needed.
func copyFileTree(src, dst string) {
	Expect(filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(src, p)
		if relErr != nil {
			return relErr
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, readErr := os.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(target, content, 0o644)
	})).To(Succeed())
}

// copyLifecycleSentinelFixture copies the checked-in lifecycle-sentinel
// fixture into a fresh directory under destParent, preserving its
// "@1.2.3"-suffixed leaf directory name (lifecycleSentinelFixtureDir's own
// doc comment explains why that suffix must survive the copy). Installing
// straight from the checked-in fixture would let a real suppression
// regression write lifecycleSentinelFile into this repository's own
// worktree instead of failing the spec, rather than into a disposable copy.
func copyLifecycleSentinelFixture(destParent string) (fixtureDir string) {
	fixtureDir = filepath.Join(destParent, filepath.Base(lifecycleSentinelFixtureDir()))
	copyFileTree(lifecycleSentinelFixtureDir(), fixtureDir)
	return fixtureDir
}

// anyFileNamed reports whether any file (not directory) named name exists
// anywhere under root, recursively. A missing root is not an error -- it
// simply contains nothing.
func anyFileNamed(root, name string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && d.Name() == name {
			found = true
		}
		return nil
	})
	return found
}

// npmGlobalStyleInstallPresent reports whether root contains an
// npm-global-style "lib/node_modules/<name>" layout anywhere under it -- the
// shape only mise's npm.shell_out=true backend's real `npm install -g`
// produces, never the default aube backend's own installer. It distinguishes
// a genuine invocation of the shell_out=true branch from a config.toml typo
// that silently fell back to the default backend, which would still pass a
// lifecycle-script-suppression assertion trivially (it never runs lifecycle
// scripts at all, regardless of this package's own suppression env var).
func npmGlobalStyleInstallPresent(root, name string) bool {
	found := false
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() || d.Name() != "lib" {
			return nil
		}
		if _, statErr := os.Stat(filepath.Join(path, "node_modules", name)); statErr == nil {
			found = true
		}
		return nil
	})
	return found
}

// filteredEnviron returns os.Environ() with every entry whose key equals key
// removed, so a positive control that must run genuinely unsuppressed cannot
// accidentally inherit that suppression from this process's own ambient
// environment.
func filteredEnviron(key string) []string {
	prefix := key + "="
	filtered := make([]string, 0, len(os.Environ()))
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, prefix) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// assertLifecycleSentinelFixtureIsLive proves the lifecycle-sentinel
// fixture's pre/postinstall scripts are genuinely capable of firing, by
// installing a disposable copy of it with a real `npm install -g` that
// carries neither mise's own npm-backend --ignore-scripts flag nor Coach's
// own suppression env var (npm_config_ignore_scripts is explicitly
// stripped from the ambient environment first, in case the host happens to
// set it). Without this, a negative "the sentinel never appears" assertion
// elsewhere in the same spec would pass just as easily if the fixture were
// inert -- AC-15's false-green-resistant sentinel requirement.
func assertLifecycleSentinelFixtureIsLive(destParent string) {
	liveCopy := copyLifecycleSentinelFixture(destParent)
	prefix := filepath.Join(destParent, "npm-global-prefix")
	Expect(os.MkdirAll(prefix, 0o755)).To(Succeed())

	cmd := exec.Command("npm", "install", "-g", "file:"+liveCopy, "--prefix", prefix)
	cmd.Env = filteredEnviron(miseNpmScriptSuppressionEnvKey)
	output, err := cmd.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "positive control npm install: %s", output)
	Expect(anyFileNamed(liveCopy, lifecycleSentinelFile)).To(BeTrue(), "expected the fixture's pre/postinstall script to fire when nothing suppresses it, or the negative assertion this spec makes elsewhere would be vacuous")
}

// freshMiseInstallEnv points HOME/MISE_DATA_DIR/MISE_CONFIG_DIR at fresh,
// empty temp directories for the duration of the current spec, so a real
// `mise install`/`mise trust` run against a local fixture package never
// touches this host's actual mise install store or trust database.
func freshMiseInstallEnv() (dataDir, configDir string) {
	home := GinkgoT().TempDir()
	dataDir = GinkgoT().TempDir()
	configDir = GinkgoT().TempDir()
	GinkgoT().Setenv("HOME", home)
	GinkgoT().Setenv("MISE_DATA_DIR", dataDir)
	GinkgoT().Setenv("MISE_CONFIG_DIR", configDir)
	return dataDir, configDir
}

var _ = Describe("mise npm-backend install command construction: lifecycle-script suppression (coach#328 Task 4, AC-4/AC-15)", func() {
	When("mise's default (aube) backend installs a copy of a local fixture package whose pre/postinstall scripts would touch a sentinel file", func() {
		It("never runs the fixture's lifecycle scripts, checked at both the file: package's own real landing site and under MISE_DATA_DIR", func() {
			if _, err := exec.LookPath("mise"); err != nil {
				Skip(fmt.Sprintf("mise not found on PATH; skipping the real mise install lifecycle-suppression spec (%s)", err))
			}
			if _, err := exec.LookPath("npm"); err != nil {
				Skip(fmt.Sprintf("npm not found on PATH; skipping the real mise install lifecycle-suppression spec (%s)", err))
			}
			dataDir, _ := freshMiseInstallEnv()
			assertLifecycleSentinelFixtureIsLive(GinkgoT().TempDir())

			// A `file:` package's lifecycle scripts run with cwd set to the
			// package's own source directory, not anywhere under
			// MISE_DATA_DIR (verified empirically) -- so a real
			// suppression regression must be caught at that directory, not
			// only under MISE_DATA_DIR. Installing from a fresh copy rather
			// than the checked-in fixture keeps that real landing site
			// disposable.
			fixtureDir := copyLifecycleSentinelFixture(GinkgoT().TempDir())

			attempted, observed, _, exitErr := runMiseInstallInsulated(context.Background(), "npm:file:"+fixtureDir)
			Expect(attempted).To(BeTrue(), "expected the install subprocess to actually start (mise confined and reachable)")
			Expect(observed).To(BeTrue(), "expected the install subprocess's outcome to be observable")
			Expect(exitErr).NotTo(HaveOccurred(), "mise install of the local fixture package must itself succeed")

			Expect(anyFileNamed(fixtureDir, lifecycleSentinelFile)).To(BeFalse(), "the fixture's pre/postinstall script must never run under mise's npm backend suppression, checked at its real landing site (the file: package's own source directory)")
			Expect(anyFileNamed(dataDir, lifecycleSentinelFile)).To(BeFalse(), "the fixture's pre/postinstall script must never run under mise's npm backend suppression")
		})
	})

	When("mise's npm.shell_out setting forces it to shell out to a real npm for the install", func() {
		It("still never runs the fixture's lifecycle scripts under that real npm invocation", func() {
			if _, err := exec.LookPath("mise"); err != nil {
				Skip(fmt.Sprintf("mise not found on PATH; skipping the mise npm.shell_out=true lifecycle-suppression spec (%s)", err))
			}
			if _, err := exec.LookPath("npm"); err != nil {
				Skip(fmt.Sprintf("npm not found on PATH; skipping the mise npm.shell_out=true lifecycle-suppression spec (%s)", err))
			}
			dataDir, configDir := freshMiseInstallEnv()
			assertLifecycleSentinelFixtureIsLive(GinkgoT().TempDir())
			// Exactly "config.toml" -- verified empirically that real mise
			// silently ignores the same [settings] table written to
			// "settings.toml" instead.
			Expect(os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("[settings]\nnpm.shell_out = true\n"), 0o644)).To(Succeed())

			fixtureDir := copyLifecycleSentinelFixture(GinkgoT().TempDir())

			attempted, observed, _, exitErr := runMiseInstallInsulated(context.Background(), "npm:file:"+fixtureDir)
			Expect(attempted).To(BeTrue(), "expected the install subprocess to actually start (mise confined and reachable)")
			Expect(observed).To(BeTrue(), "expected the install subprocess's outcome to be observable")
			Expect(exitErr).NotTo(HaveOccurred(), "mise install of the local fixture package must itself succeed under the shell_out=true backend")

			Expect(npmGlobalStyleInstallPresent(dataDir, "coach-lifecycle-sentinel")).To(BeTrue(), "expected the npm.shell_out=true backend's real `npm install -g` layout under MISE_DATA_DIR, proving this spec actually reached that branch rather than silently falling back to the default backend")

			Expect(anyFileNamed(fixtureDir, lifecycleSentinelFile)).To(BeFalse(), "the fixture's pre/postinstall script must never run under the real npm invocation mise's shell_out=true backend shells out to")
			Expect(anyFileNamed(dataDir, lifecycleSentinelFile)).To(BeFalse(), "the fixture's pre/postinstall script must never run under the real npm invocation mise's shell_out=true backend shells out to")
		})
	})
})

var _ = Describe("mise npm-backend install command construction: insulated working directory (coach#328 Task 4, AC-16)", func() {
	When("the analyzed repository's mise.toml carries a trusted env exec template that would write a sentinel", func() {
		It("never executes it, because the real install runs from a private neutral working directory that never discovers the repository's config", func() {
			if _, err := exec.LookPath("mise"); err != nil {
				Skip(fmt.Sprintf("mise not found on PATH; skipping the real mise insulated-working-directory spec (%s)", err))
			}
			freshMiseInstallEnv()

			repo := GinkgoT().TempDir()
			sentinel := filepath.Join(repo, "mise-config-side-effect")
			decoy := fmt.Sprintf("[env]\nSIDE_EFFECT = \"{{ exec(command='touch %s') }}\"\n", sentinel)
			Expect(os.WriteFile(filepath.Join(repo, "mise.toml"), []byte(decoy), 0o644)).To(Succeed())
			// hasMiseConfigHazard deliberately does not flag [env] exec()
			// templates (AC-9's framing only scopes [hooks]/[registry]/
			// _.source/tool-level execution overrides), so this decoy
			// would pass Coach's own trust gate; this spec is about the
			// separate insulation guarantee (AC-16), not the hazard scan.
			Expect(hasMiseConfigHazard(decoy)).To(BeFalse(), "sanity: this decoy construct must not be one hasMiseConfigHazard already refuses on, or this spec would not isolate AC-16's insulation guarantee")

			trustCmd := exec.Command("mise", "trust")
			trustCmd.Dir = repo
			trustOut, trustErr := trustCmd.CombinedOutput()
			Expect(trustErr).NotTo(HaveOccurred(), "mise trust: %s", trustOut)

			fixtureDir := copyLifecycleSentinelFixture(GinkgoT().TempDir())

			// Positive control: proves the decoy's env exec() template is
			// live and genuinely fires for a real mise invocation whose
			// working directory is the repository -- the exact regression
			// AC-16 exists to catch -- so the negative assertion below
			// cannot pass merely because the decoy never fires at all.
			controlCmd := exec.Command("mise", "install", "npm:file:"+fixtureDir)
			controlCmd.Dir = repo
			controlOut, controlErr := controlCmd.CombinedOutput()
			Expect(controlErr).NotTo(HaveOccurred(), "mise install (positive control, cwd=repo): %s", controlOut)
			Expect(sentinel).To(BeAnExistingFile(), "expected the repository's mise.toml env exec template to fire for a real mise invocation whose working directory is the repository itself")
			Expect(os.Remove(sentinel)).To(Succeed())

			// The actual assertion: runMiseInstallInsulated must never let
			// mise discover the repository's config at all, because its own
			// working directory is a private, neutral temp directory, never
			// the repository being analyzed.
			attempted, observed, _, exitErr := runMiseInstallInsulated(context.Background(), "npm:file:"+fixtureDir)
			Expect(attempted).To(BeTrue())
			Expect(observed).To(BeTrue())
			Expect(exitErr).NotTo(HaveOccurred())

			_, statErr := os.Stat(sentinel)
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "the repository's mise.toml env exec template must never execute during the insulated install")
		})
	})
})

// writeNoOpStubMise writes a `mise` executable that exits 0 for any
// invocation without inspecting its arguments. The trust-gate and
// compiler-verification specs below drive every mise-facing decision
// through this package's own overridable probe vars (probeMiseToolVersion,
// probeMiseGlobalConfigHazard, locateMiseTypescriptInstall) rather than a
// real mise subprocess; the one exception is runMiseInstallInsulated's own
// `mise install` call, which is not a var and always really executes --
// this stub exists only to satisfy that one call cheaply and
// deterministically, without a real (network-dependent) npm:typescript
// install.
func writeNoOpStubMise() (dir string) {
	dir = GinkgoT().TempDir()
	Expect(os.WriteFile(filepath.Join(dir, "mise"), []byte("#!/bin/sh\nexit 0\n"), 0o755)).To(Succeed())
	return dir
}

// writeFakeInstalledTypescript writes a minimal installed-package layout
// under installDir/node_modules/typescript (plus its matching native
// platform package) that classifyCompilerCandidate's own filesystem reads
// (readTypescriptVersionAt, resolveNativePackage) accept as
// compilerClassEligible for version.
func writeFakeInstalledTypescript(installDir, version string) {
	pkgDir := filepath.Join(installDir, "node_modules", "typescript")
	Expect(os.MkdirAll(pkgDir, 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(fmt.Sprintf(`{"name":"typescript","version":%q}`+"\n", version)), 0o644)).To(Succeed())

	nativeDir := filepath.Join(installDir, "node_modules", "@typescript", nativeTypescriptUnscopedName())
	Expect(os.MkdirAll(nativeDir, 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(nativeDir, "package.json"), []byte(fmt.Sprintf(`{"name":%q,"version":%q}`+"\n", NativeTypescriptPackageName(), version)), 0o644)).To(Succeed())
}

// writeStdoutOverflowStubMise writes a `mise` executable that always writes
// more than maxMiseProbeOutput bytes to stdout before exiting 0, regardless
// of its arguments -- exercising runBoundedMiseInstallSubprocess's
// stdout-budget-overflow branch (project_ts_compiler_mise_command.go's
// `int64(len(data)) > maxMiseProbeOutput` check) deterministically, without
// a real, network-dependent, minutes-long mise install.
func writeStdoutOverflowStubMise() (dir string) {
	dir = GinkgoT().TempDir()
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%%ds' '' | tr ' ' 'x'\nexit 0\n", maxMiseProbeOutput+1024)
	Expect(os.WriteFile(filepath.Join(dir, "mise"), []byte(script), 0o755)).To(Succeed())
	return dir
}

// writeNonZeroExitStubMise writes a `mise` executable that exits 1 for any
// invocation without inspecting its arguments -- exercising
// installMiseTypescript's just-ran-but-failed branch (a fully observed
// subprocess whose own exit was non-zero) deterministically, without a real
// mise install.
func writeNonZeroExitStubMise() (dir string) {
	dir = GinkgoT().TempDir()
	Expect(os.WriteFile(filepath.Join(dir, "mise"), []byte("#!/bin/sh\nexit 1\n"), 0o755)).To(Succeed())
	return dir
}

var _ = Describe("mise npm-backend install command construction: post-launch failure classification (coach#328 Task 4, AC-SET-7)", func() {
	When("the install subprocess starts but exceeds the stdout budget before exiting", func() {
		It("reports attempted=true, observed=false -- disk state may already have changed even though the outcome could not be read back", func() {
			stubDir := writeStdoutOverflowStubMise()
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

			attempted, observed, _, exitErr := runMiseInstallInsulated(context.Background(), "npm:file:does-not-matter")
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
		originalProbeVersion = probeMiseToolVersion
		originalGlobalHazard = probeMiseGlobalConfigHazard
		originalLocate = locateMiseTypescriptInstall
		probeMiseToolVersion = func(context.Context) (string, bool) { return "2026.9.5 linux-x64 (2026-09-10)", true }
		probeMiseGlobalConfigHazard = func(context.Context) bool { return false }
	})

	AfterEach(func() {
		probeMiseToolVersion = originalProbeVersion
		probeMiseGlobalConfigHazard = originalGlobalHazard
		locateMiseTypescriptInstall = originalLocate
	})

	When("the project mise.toml carries an execution hazard ([hooks])", func() {
		It("refuses with package_manager_config_unverifiable before ever attempting mise install", func() {
			repo := GinkgoT().TempDir()
			Expect(os.WriteFile(filepath.Join(repo, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"7.0.2\"\n\n[hooks]\npostinstall = \"echo pwned\"\n"), 0o644)).To(Succeed())

			result := installMiseTypescriptProject(context.Background(), repo, "7.0.2")
			Expect(result.Trusted).To(BeFalse(), "a hazardous project mise.toml must refuse before ever running mise: %+v", result)
			Expect(result.Attempted).To(BeFalse(), "an untrusted scope must never attempt mise install: %+v", result)
			Expect(result.Code).To(Equal(GapPackageManagerConfigUnverifiable))
		})
	})

	When("the trusted scope's private working directory cannot be created (coach#328 Task 5 integration repair, Finding 5)", func() {
		It("reports package_manager_config_unverifiable rather than a bare, codeless could-not-start failure", func() {
			// Every GinkgoT().TempDir() call is a fresh os.MkdirTemp("", "ginkgo")
			// (no per-spec caching), so both temp directories this spec needs
			// must be created before TMPDIR is pointed at a nonexistent path --
			// otherwise a later, unrelated TempDir() call would fail instead of
			// the production code path this spec actually targets.
			repo := GinkgoT().TempDir()
			Expect(os.WriteFile(filepath.Join(repo, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"7.0.2\"\n"), 0o644)).To(Succeed())
			nonexistentTemp := filepath.Join(GinkgoT().TempDir(), "does-not-exist")
			GinkgoT().Setenv("TMPDIR", nonexistentTemp)

			result := installMiseTypescriptProject(context.Background(), repo, "7.0.2")
			Expect(result.Trusted).To(BeTrue(), "the trust gate itself never touches the working directory: %+v", result)
			Expect(result.Attempted).To(BeFalse(), "the install subprocess must never start when insulation could not be established: %+v", result)
			Expect(result.Observed).To(BeFalse(), "%+v", result)
			Expect(result.Code).To(Equal(GapPackageManagerConfigUnverifiable), "insulation failing before any subprocess starts must be distinguishable from mise simply being absent from PATH: %+v", result)
		})
	})

	When("the project mise scope is trusted and mise reports a successful install", func() {
		It("verifies the installed compiler via classifyCompilerCandidate and reports it eligible", func() {
			stubDir := writeNoOpStubMise()
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

			installDir := GinkgoT().TempDir()
			writeFakeInstalledTypescript(installDir, "7.0.2")
			locateMiseTypescriptInstall = func(context.Context, string) (string, bool) {
				return filepath.Join(installDir, "node_modules", "typescript"), true
			}

			repo := GinkgoT().TempDir()
			Expect(os.WriteFile(filepath.Join(repo, "mise.toml"), []byte("[tools]\n\"npm:typescript\" = \"7.0.2\"\n"), 0o644)).To(Succeed())

			result := installMiseTypescriptProject(context.Background(), repo, "7.0.2")
			Expect(result.Trusted).To(BeTrue(), "%+v", result)
			Expect(result.Attempted).To(BeTrue(), "%+v", result)
			Expect(result.Succeeded).To(BeTrue(), "expected the freshly-installed compiler to classify as eligible: %+v", result)
			Expect(result.Class).To(Equal(compilerClassEligible))
			Expect(result.Version).To(Equal("7.0.2"))
			Expect(result.Origin).To(Equal(compilerOriginMiseProject))
		})
	})

	When("the global mise scope is trusted and mise reports a successful install", func() {
		It("verifies the installed compiler via classifyCompilerCandidate and reports it eligible", func() {
			stubDir := writeNoOpStubMise()
			GinkgoT().Setenv("PATH", stubDir+string(os.PathListSeparator)+os.Getenv("PATH"))

			installDir := GinkgoT().TempDir()
			writeFakeInstalledTypescript(installDir, "7.0.2")
			locateMiseTypescriptInstall = func(context.Context, string) (string, bool) {
				return filepath.Join(installDir, "node_modules", "typescript"), true
			}

			result := installMiseTypescriptGlobal(context.Background(), "7.0.2")
			Expect(result.Trusted).To(BeTrue(), "%+v", result)
			Expect(result.Attempted).To(BeTrue(), "%+v", result)
			Expect(result.Succeeded).To(BeTrue(), "expected the freshly-installed compiler to classify as eligible: %+v", result)
			Expect(result.Origin).To(Equal(compilerOriginMiseGlobal))
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

			// No writeFakeInstalledTypescript call: locateMiseTypescriptInstall
			// finds nothing at all, so classifyCompilerCandidate classifies
			// compilerClassAbsent -- a genuinely observed, exit-zero install
			// whose freshly-installed compiler still fails verification,
			// exactly the case Finding 2's CLI message must never describe as
			// "mise install failed".
			locateMiseTypescriptInstall = func(context.Context, string) (string, bool) {
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

var _ = Describe("mise npm-backend install command construction: real end-to-end native-package classification (coach#328 Task 5 integration repair, Finding 1)", func() {
	When("mise's default npm backend genuinely installs the frozen row's TypeScript version, with no fabricated filesystem layout standing in for it", func() {
		It("classifies the freshly-installed compiler as eligible, not merely as an install that exited zero", func() {
			if _, err := exec.LookPath("mise"); err != nil {
				Skip(fmt.Sprintf("mise not found on PATH; skipping the real end-to-end mise install classification spec (%s)", err))
			}
			freshMiseInstallEnv()

			result := installMiseTypescriptGlobal(context.Background(), "7.0.2")
			Expect(result.Trusted).To(BeTrue(), "expected the fresh, hazard-free global mise scope to be trusted: %+v", result)
			Expect(result.Attempted).To(BeTrue(), "expected a real `mise install` subprocess to actually run: %+v", result)
			Expect(result.Succeeded).To(BeTrue(), "a real `mise install npm:typescript@7.0.2` must classify as compilerClassEligible, not merely exit zero -- coach#328 Finding 1: mise's own default npm backend does not hoist the platform-native optionalDependency package to the top-level location classifyCompilerCandidate expects unless something completes that hoisting: %+v", result)
			Expect(result.Class).To(Equal(compilerClassEligible), "%+v", result)
			Expect(result.NativePath).NotTo(BeEmpty(), "%+v", result)
		})
	})
})
