package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --baseline --check-project: read-only mise probe confinement", func() {
	When("a read-only mise probe runs during --check-project", func() {
		It("uses a private per-invocation working directory, not a fixed shared path any local user could plant configuration in", func() {
			repo := newTempGitRepo()
			commitFile(repo, "project.json", singleRootPolicyJSON)
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "mise.toml", "[tools]\n\"npm:typescript\" = \"7.0.2\"\n")

			path, miseDir := pathWithStubMiseDefaultTool("v24.9.9", "7.0.2")

			_, stderr, exitCode := runCoachCheckProjectEnv(repo, path, "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "project.json", "--format", "json")
			Expect(exitCode).To(Equal(0), "stderr: %s", stderr)

			shared := filepath.Join(os.TempDir(), "coach-mise-probe")
			cwds := readStubMiseCwds(miseDir)
			Expect(cwds).NotTo(BeEmpty())
			for _, cwd := range cwds {
				Expect(cwd).NotTo(Equal(repo), "a probe must never run with the analyzed repository as cwd, got %q", cwd)
				Expect(cwd).NotTo(Equal(shared), "a probe must not run in a predictable shared directory another local user can pre-create, got %q", cwd)
				Expect(cwd).NotTo(BeADirectory(), "the private probe directory must be removed after the probe, %q still exists", cwd)
			}
		})
	})
})
