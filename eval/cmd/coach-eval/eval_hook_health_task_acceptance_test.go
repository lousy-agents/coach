package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the eval-hook-health mise task (#789, design 10)", Label(typeScriptLabel), func() {
	It("checks the hook and the probe on the Go and the TypeScript tuning cells and exits 0", func() {
		cmd := exec.Command("mise", "run", "eval-hook-health")
		cmd.Dir = repositoryRoot()
		cmd.Env = append(cmd.Environ(), "XDG_CACHE_HOME="+GinkgoT().TempDir())

		outcome := runGenerateCommand(cmd)

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		Expect(outcome.stdout).To(ContainSubstring("ok "))
	})

	It("installs the pinned compiler first and builds the coach it runs from this checkout (structural)", func() {
		config, err := os.ReadFile(filepath.Join(repositoryRoot(), "mise.toml"))
		Expect(err).NotTo(HaveOccurred())

		task := regexp.MustCompile(`(?s)\[tasks\.eval-hook-health\](.*?)(\n\[|\z)`).FindSubmatch(config)
		Expect(task).NotTo(BeNil(), "mise.toml defines eval-hook-health")
		Expect(string(task[1])).To(MatchRegexp(`depends\s*=\s*\[[^\]]*"js-install"`))
		Expect(string(task[1])).To(ContainSubstring("./cmd/coach"))
		Expect(string(task[1])).To(ContainSubstring("hook-health"))
	})
})
