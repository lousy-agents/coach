package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

var commandPath string

var _ = BeforeSuite(func() {
	directory, err := os.MkdirTemp("", "acceptance-guard-preflight-acceptance-*")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, directory)
	commandPath = filepath.Join(directory, "acceptance-guard-preflight")
	build := exec.Command("go", "build", "-o", commandPath, ".")
	output, err := build.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), "building the command: %s", output)
})

// cleanEnviron returns the current process environment with every known
// ambient-credential variable stripped and HOME repointed at home, so a
// spawned acceptance-guard-preflight subprocess never observes this test
// runner's own real ambient environment (or a real ~/.aws/credentials on
// the host running this suite) regardless of what's actually present.
func cleanEnviron(home string) []string {
	ambient := make(map[string]bool, len(acceptanceharness.AmbientCredentialVars))
	for _, name := range acceptanceharness.AmbientCredentialVars {
		ambient[name] = true
	}

	var out []string
	for _, kv := range os.Environ() {
		name, _, ok := strings.Cut(kv, "=")
		if !ok || ambient[name] || name == "HOME" {
			continue
		}
		out = append(out, kv)
	}
	return append(out, "HOME="+home)
}

var _ = Describe("acceptance-guard-preflight command", func() {
	When("run with a tainted environment (a known ambient-credential variable set)", func() {
		It("exits non-zero and writes a diagnostic to stderr", func() {
			home := GinkgoT().TempDir()
			cmd := exec.Command(commandPath)
			cmd.Env = append(cleanEnviron(home), "GITHUB_TOKEN=ghp_totallyfake")
			var stderr strings.Builder
			cmd.Stderr = &stderr

			err := cmd.Run()

			Expect(err).To(HaveOccurred())
			var exitErr *exec.ExitError
			Expect(err).To(BeAssignableToTypeOf(exitErr))
			Expect(stderr.String()).To(ContainSubstring("GITHUB_TOKEN"))
		})
	})

	When("run with a clean environment (no ambient credentials, no default credential file)", func() {
		It("exits zero and writes nothing to stderr", func() {
			home := GinkgoT().TempDir()
			cmd := exec.Command(commandPath)
			cmd.Env = cleanEnviron(home)
			var stderr strings.Builder
			cmd.Stderr = &stderr

			err := cmd.Run()

			Expect(err).NotTo(HaveOccurred(), "stderr: %s", stderr.String())
			Expect(stderr.String()).To(BeEmpty())
		})
	})
})
