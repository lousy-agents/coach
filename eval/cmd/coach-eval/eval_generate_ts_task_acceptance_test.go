package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the eval-generate mise task for a TypeScript program", func() {
	It("reaches the generate command with --language (781)", func() {
		cmd := exec.Command("mise", "run", "eval-generate", "--", "--language", "typescript", "--cell", "0", "--seed", "1")
		cmd.Dir = repositoryRoot()
		cmd.Env = append(cmd.Environ(), "XDG_CACHE_HOME="+GinkgoT().TempDir())

		outcome := runGenerateCommand(cmd)

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		Expect(outcome.stdout).To(ContainSubstring(`"program_id"`))
	})

	It("installs the pinned compiler first, because the generator resolves it from js/semantics (structural)", func() {
		config, err := os.ReadFile(filepath.Join(repositoryRoot(), "mise.toml"))
		Expect(err).NotTo(HaveOccurred())

		task := regexp.MustCompile(`(?s)\[tasks\.eval-generate\](.*?)(\n\[|\z)`).FindSubmatch(config)
		Expect(task).NotTo(BeNil())
		Expect(string(task[1])).To(MatchRegexp(`depends\s*=\s*\[[^\]]*"js-install"`))
	})
})
