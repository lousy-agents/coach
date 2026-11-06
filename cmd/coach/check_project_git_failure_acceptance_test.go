package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal --check-project: a git failure while judging an explicit policy is an operational failure", func() {
	When("the policy is gone from disk and the object that holds its committed content is gone from the repository", func() {
		It("exits 1 naming the path and writes no readiness result, instead of reporting a clean-looking policy_missing", func() {
			repo := newTempGitRepo()
			commitFile(repo, "package.json", `{"name":"example","version":"1.0.0"}`+"\n")
			commitFile(repo, "tsconfig.json", `{"compilerOptions":{}}`+"\n")
			commitFile(repo, "cfg/p.json", singleRootPolicyJSON)
			blob := strings.TrimSpace(gitIn(repo, "rev-parse", "HEAD:cfg/p.json"))
			Expect(os.Remove(filepath.Join(repo, ".git", "objects", blob[:2], blob[2:]))).To(Succeed(), "the loose object must exist in a fresh repository")
			Expect(os.RemoveAll(filepath.Join(repo, "cfg"))).To(Succeed())

			stdout, stderr, exitCode := runCoachCheckProjectEnv(repo, pathWithoutNode(), "--baseline", "--check-project", "--project-config", "cfg/p.json", "--project-language", "typescript", "--format", "json")

			Expect(exitCode).To(Equal(1), "git could not answer, which is an operational failure; stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty(), "no readiness result may be written when the check could not be completed; stdout=%s", stdout)
			Expect(string(stderr)).To(ContainSubstring("cfg/p.json"), "stderr: %s", stderr)
		})
	})
})
