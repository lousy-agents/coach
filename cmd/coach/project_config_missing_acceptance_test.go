package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const missingPolicyPath = "policies/absent.json"

var _ = Describe("coach codesignal: an explicit --project-config that does not exist on disk", func() {
	When("the scan starts in a subdirectory", func() {
		var repo, subdir string

		BeforeEach(func() {
			repo = commitScannableGoRepo()
			subdir = filepath.Join(repo, "nested")
			Expect(os.MkdirAll(subdir, 0o755)).To(Succeed())
		})

		It("explains a file that exists uncommitted at the repository root as uncommitted, not as not found", func() {
			writeWorktreeFile(repo, "unc.json", singleRootPolicyJSON)

			stdout, stderr, exitCode := runCoachBinary(commandPath, subdir, nil, "codesignal", "--baseline", "--project-config", "unc.json", "--format=json")

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("exists in the worktree but is not committed"))
		})
	})
})
