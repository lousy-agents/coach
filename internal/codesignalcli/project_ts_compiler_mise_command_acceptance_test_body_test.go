package codesignalcli

import (
	"context"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func body_projectTsCompilerMiseCommandAcceptanceTest_neverRunsTheFixtureSLifecycleScriptsCheckedAtThe_40() {
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

func body_projectTsCompilerMiseCommandAcceptanceTest_stillNeverRunsTheFixtureSLifecycleScriptsUnderTh_62() {
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

func body_projectTsCompilerMiseCommandAcceptanceTest_neverExecutesItBecauseTheRealInstallRunsFromAPri_90() {
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

func body_projectTsCompilerMiseCommandAcceptanceTest_classifiesTheFreshlyInstalledCompilerAsEligibleN_281() {
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

func body_projectTsCompilerMiseCommandAcceptanceTest_298(p string, d fs.DirEntry, err error, src string, dst string) error {
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
}

func body_projectTsCompilerMiseCommandAcceptanceTest_349(w http.ResponseWriter, r *http.Request, tarball []byte, tarballPath string, packument []byte, versionJSON []byte) {
	path := strings.TrimSuffix(r.URL.Path, "/")
	switch path {
	case "/" + lifecycleSentinelPackageName:
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(packument)
	case "/" + lifecycleSentinelPackageName + "/" + lifecycleSentinelVersion:
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(versionJSON)
	case tarballPath:
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(tarball)
	default:
		GinkgoWriter.Printf("lifecycle-sentinel registry: unexpected %s %s\n", r.Method, r.URL.Path)
		http.NotFound(w, r)
	}
}
