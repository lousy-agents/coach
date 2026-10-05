package tssetup

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// anyFileNamed: a missing root is not an error -- it simply contains
// nothing.
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

func writeHomeNpmrcRegistry(registryURL string) {
	home := os.Getenv("HOME")
	Expect(home).NotTo(BeEmpty(), "freshMiseInstallEnv must run before writing HOME/.npmrc")
	if !strings.HasSuffix(registryURL, "/") {
		registryURL += "/"
	}
	Expect(os.WriteFile(filepath.Join(home, ".npmrc"), []byte("registry="+registryURL+"\n"), 0o644)).To(Succeed())
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

// writeNoOpStubMise writes a `mise` executable that exits 0 for any
// invocation without inspecting its arguments. The trust-gate and
// compiler-verification specs below drive every mise-facing decision
// through this package's own overridable probe vars (tstoolchain.ProbeMiseToolVersion,
// tstoolchain.ProbeMiseGlobalConfigHazard, tstoolchain.LocateMiseTypescriptInstall) rather than a
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
// platform package) that tstoolchain.ClassifyCandidate's own filesystem reads
// (readTypescriptVersionAt, resolveNativePackage) accept as
// tstoolchain.ClassEligible for version.
func writeFakeInstalledTypescript(installDir, version string) {
	pkgDir := filepath.Join(installDir, "node_modules", "typescript")
	Expect(os.MkdirAll(pkgDir, 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(pkgDir, "package.json"), []byte(fmt.Sprintf(`{"name":"typescript","version":%q}`+"\n", version)), 0o644)).To(Succeed())

	nativeDir := filepath.Join(installDir, "node_modules", "@typescript", tstoolchain.NativeTypescriptUnscopedName())
	Expect(os.MkdirAll(nativeDir, 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(nativeDir, "package.json"), []byte(fmt.Sprintf(`{"name":%q,"version":%q}`+"\n", tstoolchain.NativeTypescriptPackageName(), version)), 0o644)).To(Succeed())
}

// writeStdoutOverflowStubMise writes a `mise` executable that always writes
// more than tstoolchain.MaxMiseProbeOutput bytes to stdout before exiting 0, regardless
// of its arguments -- exercising runBoundedMiseInstallSubprocess's
// stdout-budget-overflow branch (project_ts_compiler_mise_command.go's
// `int64(len(data)) > tstoolchain.MaxMiseProbeOutput` check) deterministically, without
// a real, network-dependent, minutes-long mise install.
func writeStdoutOverflowStubMise() (dir string) {
	dir = GinkgoT().TempDir()
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%%ds' '' | tr ' ' 'x'\nexit 0\n", tstoolchain.MaxMiseProbeOutput+1024)
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
