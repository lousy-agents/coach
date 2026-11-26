package main_test

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// pathWithGoAndNode is a PATH on which the TypeScript path finds the tools it runs and `coach` does not resolve.
func pathWithGoAndNode() string {
	directory := GinkgoT().TempDir()
	for _, tool := range []string{"go", "node"} {
		binary, err := exec.LookPath(tool)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Symlink(binary, filepath.Join(directory, tool))).To(Succeed())
	}
	return directory
}

var _ = Describe("coach-eval generate records what Coach reports on a TypeScript base", Label(typeScriptLabel), func() {
	args := []string{"--language", "typescript", "--cell", "0", "--seed", "1"}

	It("records coach_base_fp null with the reason when there is no coach binary, and still emits the program (AC-22)", func() {
		outcome := runGenerate(GinkgoT().TempDir(), []string{"PATH=" + pathWithGoAndNode(), "COACH_BIN="}, args...)
		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)

		count, reason := baseFindings(decodeGenerated(outcome.stdout))

		Expect(count).To(BeNil())
		Expect(reason).To(Equal("no coach binary"))
	})

	It("records the number of signals a coach run the way the design requires reports, never rejecting for them (AC-22)", func() {
		coach := strictCoach(GinkgoT().TempDir(), `{"signals":[{},{}]}`)

		outcome := runGenerate(GinkgoT().TempDir(), []string{"COACH_BIN=" + coach}, args...)
		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		count, reason := baseFindings(decodeGenerated(outcome.stdout))

		Expect(count).To(BeEquivalentTo(2))
		Expect(reason).To(BeNil())
	})
})
