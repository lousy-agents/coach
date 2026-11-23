package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// ADR-009 reads the committed copy only for disclosure, so a valid policy on disk should win over an invalid or
// linked committed one. These specs pin what a scan does today, so the A2b change that flips them is deliberate.
var _ = Describe("coach codesignal: a canonical policy committed unusable, with a valid policy on disk (#168 A2a, interim until A2b)", func() {
	for _, mode := range scopePolicyModes {
		It("#168 A2a, interim until A2b: invalid committed content with a valid policy on disk still exits 2 under "+mode.name+" and tells the user to commit the fix", func() {
			repo, base := scopePolicyRepo(scopePolicyFile{coachConfigPath, invalidPolicyText})
			scopePolicyRewrite(repo, coachConfigPath, singleRootPolicyJSON)
			Expect(runGitInRepo(repo, "show", "HEAD:"+coachConfigPath)).To(Equal(invalidPolicyText), "fixture: the committed policy is invalid")
			onDisk, err := os.ReadFile(filepath.Join(repo, coachConfigPath))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(onDisk)).To(Equal(singleRootPolicyJSON), "fixture: the policy on disk is valid")

			stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
			Expect(string(stderr)).To(ContainSubstring("commit it if it is the fix"))
		})

		It("#168 A2a, interim until A2b: a canonical policy committed as a symbolic link with a valid regular file on disk still exits 2 under "+mode.name+" and tells the user to commit the fix", func() {
			repo, base := scopePolicyRepo(scopePolicyFile{"real.json", singleRootPolicyJSON})
			symlinkAt(repo, coachConfigPath)
			gitIn(repo, "add", coachConfigPath)
			gitInWithIdentity(repo, "commit", "-q", "-m", "the canonical policy as a link")
			base = scopePolicyHead(repo)
			commitFile(repo, "c.go", "package a\n\nfunc C() {}\n")
			Expect(runGitInRepo(repo, "ls-tree", "HEAD", coachConfigPath)).To(HavePrefix("120000 "), "fixture: the committed policy is a symbolic link")
			Expect(os.Remove(filepath.Join(repo, coachConfigPath))).To(Succeed())
			scopePolicyRewrite(repo, coachConfigPath, singleRootPolicyJSON)
			info, err := os.Lstat(filepath.Join(repo, coachConfigPath))
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().IsRegular()).To(BeTrue(), "fixture: the policy on disk is a valid regular file")

			stdout, stderr, exitCode := mode.run(repo, base, "--format=json")

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(stdout).To(BeEmpty())
			Expect(string(stderr)).To(ContainSubstring("project_config_invalid"))
			Expect(string(stderr)).To(ContainSubstring("committed as a symbolic link"))
			Expect(string(stderr)).To(ContainSubstring("commit a regular file"))
		})
	}
})
