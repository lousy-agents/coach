package projectmodel_test

import (
	"os/exec"

	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	realTSSidecarOnce sync.Once
	realTSSidecarPath string
	realTSSidecarSkip string
)

var _ = Describe("BuildTypeScriptModelViaSidecar against the real compiled Node/TypeScript sidecar", Label("ts-sidecar-integration"), func() {
	body_tsSidecarIntegrationAcceptanceTest_BuildTypeScriptModelViaSidecarAgainstTheRealComp_32()
})

var _ = Describe("pkg/projectmodel's dependency boundary", func() {
	It("never imports pkg/codesignal, so this facts-only slice cannot emit a Signal", func() {
		cmd := exec.Command("go", "list", "-deps", "./pkg/projectmodel/...")
		cmd.Dir = repoRootFromThisFile()
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "go list -deps: %s", output)
		Expect(string(output)).NotTo(ContainSubstring("coach/pkg/codesignal"), "pkg/projectmodel must never depend on pkg/codesignal (see model.go's package doc)")
	})
})
