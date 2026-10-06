package projectmodel_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/lousy-agents/coach/pkg/projectmodel"
)

var (
	realTSSidecarOnce sync.Once
	realTSSidecarPath string
	realTSSidecarSkip string
)

// ensureRealTSSidecarBinary builds the real compiled sidecar
// (`npm run build:project-sidecar`, i.e. `mise run project-sidecar-build`)
// at most once per test binary run and memoizes the outcome. It cannot be a
// second Ginkgo BeforeSuite -- Ginkgo permits exactly one per suite, and
// ts_sidecar_fake_fixtures_test.go (frozen, Task 1) already declares the
// suite's only one, for the fake sidecar -- so every real-sidecar spec calls this
// from its own BeforeEach and Skips with skipReason when Node/npm are
// unavailable or the build fails, degrading this suite gracefully instead
// of failing Go-only environments (issue #214's explicit requirement).
func ensureRealTSSidecarBinary() (path string, skipReason string) {
	realTSSidecarOnce.Do(buildRealTSSidecarBinary)
	return realTSSidecarPath, realTSSidecarSkip
}

func buildRealTSSidecarBinary() {
	if _, err := exec.LookPath("node"); err != nil {
		realTSSidecarSkip = fmt.Sprintf("node not found on PATH; skipping real ts sidecar integration suite (%s)", err)
		return
	}
	if _, err := exec.LookPath("npm"); err != nil {
		realTSSidecarSkip = fmt.Sprintf("npm not found on PATH; skipping real ts sidecar integration suite (%s)", err)
		return
	}
	root := jsSemanticsRoot()
	build := exec.Command("npm", "run", "build:project-sidecar")
	build.Dir = root
	output, err := build.CombinedOutput()
	if err != nil {
		realTSSidecarSkip = fmt.Sprintf("building the real ts sidecar failed; skipping real ts sidecar integration suite: %s: %s", err, output)
		return
	}
	binPath := filepath.Join(root, "bin", "coach-ts-project-sidecar")
	if _, statErr := os.Stat(binPath); statErr != nil {
		realTSSidecarSkip = fmt.Sprintf("real ts sidecar binary missing after build; skipping: %s", statErr)
		return
	}
	realTSSidecarPath = binPath
}

func realSidecarOptions(sidecarPath string) projectmodel.TSSidecarOptions {
	compilerModule := filepath.Join(jsSemanticsRoot(), "node_modules", "typescript")
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x64"
	case "386":
		arch = "ia32"
	}
	nativePackage := filepath.Join(jsSemanticsRoot(), "node_modules", "@typescript", fmt.Sprintf("typescript-%s-%s", runtime.GOOS, arch))
	return projectmodel.TSSidecarOptions{
		BinaryPath: sidecarPath,
		Args:       []string{"--compiler-module=" + compilerModule, "--native-package=" + nativePackage},
		Timeout:    20 * time.Second,
	}
}

// pathExcludingExecutables returns the current process's PATH with any
// directory containing one of names removed, used by the "missing runtime"
// spec to construct a child environment where the sidecar's
// `#!/usr/bin/env node` shebang cannot locate node.
func pathExcludingExecutables(names ...string) string {
	var kept []string
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if dir == "" {
			continue
		}
		if !dirContainsAnyFile(dir, names) {
			kept = append(kept, dir)
		}
	}
	return strings.Join(kept, string(os.PathListSeparator))
}

func dirContainsAnyFile(dir string, names []string) bool {
	for _, name := range names {
		if info, err := os.Stat(filepath.Join(dir, name)); err == nil && !info.IsDir() {
			return true
		}
	}
	return false
}
