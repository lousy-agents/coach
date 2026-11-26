package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the eval-antipatterns mise task (#871)", func() {
	It("builds coach from this checkout, scans the fixtures and exits 0", func() {
		cmd := exec.Command("mise", "run", "eval-antipatterns")
		cmd.Dir = repositoryRoot()

		outcome := runGenerateCommand(cmd)

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		Expect(outcome.stdout).To(ContainSubstring(`"dependency-rule-breach"`))
	})

	It("builds coach from the checkout and passes the fixtures directory (structural)", func() {
		config, err := os.ReadFile(filepath.Join(repositoryRoot(), "mise.toml"))
		Expect(err).NotTo(HaveOccurred())

		task := regexp.MustCompile(`(?s)\[tasks\.eval-antipatterns\](.*?)(\n\[|\z)`).FindSubmatch(config)
		Expect(task).NotTo(BeNil(), "mise.toml defines eval-antipatterns")
		Expect(string(task[1])).To(ContainSubstring("./cmd/coach"))
		Expect(string(task[1])).To(ContainSubstring("antipatterns --coach"))
		Expect(string(task[1])).To(ContainSubstring("eval/fixtures/antipatterns"))
	})
})
