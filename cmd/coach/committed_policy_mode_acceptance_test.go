package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ADR-009 §3: only a regular file in the revision is the policy the revision holds. The entry's mode decides it,
// because a link and a file are both blobs.
var _ = Describe("coach codesignal: the mode of the policy entry the revision commits (#168 A2a)", func() {
	const policy = "cfg/p.json"

	scanText := func(repo string) string {
		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", policy)
		ExpectWithOffset(1, exitCode).To(Equal(0), "stdout=%s stderr=%s", stdout, stderr)
		return string(stdout)
	}

	When("the revision commits the policy as an executable file and the disk holds different bytes", func() {
		It("is still the committed policy, so the policy on disk is reported as differing from HEAD, not as uncommitted", func() {
			repo := commitScannableGoRepo()
			writeWorktreeFile(repo, policy, singleRootPolicyJSON)
			gitIn(repo, "add", policy)
			gitIn(repo, "update-index", "--chmod=+x", policy)
			gitInWithIdentity(repo, "commit", "-q", "-m", "an executable policy")
			writeWorktreeFile(repo, policy, goLayerPolicyConfigJSON)

			text := scanText(repo)

			Expect(text).To(ContainSubstring("differs from HEAD."))
			Expect(text).NotTo(ContainSubstring("not committed at HEAD"))
		})
	})

	When("the revision commits the policy as a symbolic link", func() {
		It("still refuses a scan when the working tree holds no file at the path", func() {
			repo := commitScannableGoRepo()
			symlinkAt(repo, policy)
			gitIn(repo, "add", policy)
			gitInWithIdentity(repo, "commit", "-q", "-m", "a policy as a link")
			Expect(os.Remove(filepath.Join(repo, policy))).To(Succeed())

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", policy)

			Expect(exitCode).To(Equal(2), "stdout=%s", stdout)
			Expect(string(stderr)).To(And(ContainSubstring("(project_config_missing)"), ContainSubstring("as a symbolic link, which a policy cannot be")))
		})
	})
})
