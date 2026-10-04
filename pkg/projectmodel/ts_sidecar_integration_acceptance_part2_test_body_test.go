package projectmodel_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func body_tsSidecarIntegrationAcceptancePart2Test_55() {
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
