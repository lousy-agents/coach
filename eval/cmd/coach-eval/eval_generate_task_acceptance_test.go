package main_test

import (
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func runGenerateThroughMise(cacheHome string, args ...string) generateOutcome {
	cmd := exec.Command("mise", append([]string{"run", "eval-generate", "--"}, args...)...)
	cmd.Dir = filepath.Join("..", "..", "..")
	cmd.Env = append(cmd.Environ(), "XDG_CACHE_HOME="+cacheHome)
	return runCommand(cmd)
}

var _ = Describe("the eval-generate mise task", func() {
	When("run with a cell and a seed", func() {
		It("reaches the generate command and prints the program (AC-11)", func() {
			outcome := runGenerateThroughMise(GinkgoT().TempDir(), "--cell", "0", "--seed", "1")

			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			Expect(outcome.stdout).To(ContainSubstring(`"program_id"`))
		})
	})

	When("the cell is out of range", func() {
		It("exits 2 through the task (AC-11)", func() {
			outcome := runGenerateThroughMise(GinkgoT().TempDir(), "--cell", "1728")

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stderr).To(ContainSubstring("cell"))
		})
	})
})
