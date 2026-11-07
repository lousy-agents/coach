package main

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: a committed explicit policy that is gone from disk is restored only when the restore overwrites nothing", func() {
	const (
		policy       = "pol.json"
		stagedPolicy = `{"schema_version":"1","roots":["staged"]}`
	)
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		commitFile(repo, policy, singleRootPolicyJSON)
	})

	scanMissing := func(env []string) (stderr []byte) {
		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, env, "codesignal", "--baseline", "--project-config", policy, "--format=json")
		ExpectWithOffset(1, exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
		ExpectWithOffset(1, string(stderr)).To(ContainSubstring("project_config_missing"))
		return stderr
	}

	expectNoRestoreNoAuthoring := func(stderr []byte) {
		ExpectWithOffset(1, string(stderr)).NotTo(ContainSubstring("git restore"), "stderr:\n%s", stderr)
		ExpectWithOffset(1, string(stderr)).NotTo(ContainSubstring(suggestCommand), "stderr:\n%s", stderr)
	}

	When("the policy has a staged edit and the file was removed from the working tree", func() {
		BeforeEach(func() {
			Expect(os.WriteFile(filepath.Join(repo, policy), []byte(stagedPolicy), 0o644)).To(Succeed())
			gitIn(repo, "add", policy)
			Expect(os.Remove(filepath.Join(repo, policy))).To(Succeed())
		})

		It("offers no restore from the commit, because it would discard the staged edit, says so, and names the restore that keeps the edit", func() {
			stderr := scanMissing(nil)

			Expect(string(stderr)).NotTo(ContainSubstring("--source"), "a restore from the commit discards the edit:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring(suggestCommand), "stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("git restore -- ':(top,literal)"+policy+"'"), "the restore from the index keeps the edit:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("staged"), "the message names why no restore from the commit is offered:\n%s", stderr)
			Expect(gitIn(repo, "show", ":"+policy)).To(Equal(stagedPolicy), "the staged edit is untouched")
		})

		It("names neither restore_policy nor author_policy and keeps the path in dirty_worktree", func() {
			doc := expectCheckProject(repo, "--project-config", policy)

			Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"))
			Expect(doc.DirtyWorktree.Paths).To(ContainElement(policy), "the staged edit was not read, so it is disclosed")
		})
	})

	When("the policy is committed as a symbolic link and the link was removed from the working tree", func() {
		const linked = "linked.json"

		BeforeEach(func() {
			Expect(os.Symlink(policy, filepath.Join(repo, linked))).To(Succeed())
			gitIn(repo, "add", linked)
			commit := exec.Command("git", "commit", "-q", "-m", "link the policy")
			commit.Dir = repo
			commit.Env = commitEnv
			output, err := commit.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "git commit: %s", output)
			Expect(os.Remove(filepath.Join(repo, linked))).To(Succeed())
		})

		It("offers no restore, which would recreate a link the scan refuses", func() {
			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", linked, "--format=json")

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			expectNoRestoreNoAuthoring(stderr)
			Expect(string(stderr)).To(ContainSubstring("symbolic link"), "stderr:\n%s", stderr)
		})

		It("names neither restore_policy nor author_policy in --check-project", func() {
			doc := expectCheckProject(repo, "--project-config", linked)

			Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("restore_policy"))
			Expect(nextActionKinds(doc)).NotTo(ContainElement("author_policy"))
		})
	})

	When("the policy is committed as a symbolic link and a file stands where its parent directory was", func() {
		const linkedNested = "cfg/linked.json"

		BeforeEach(func() {
			Expect(os.MkdirAll(filepath.Join(repo, "cfg"), 0o755)).To(Succeed())
			Expect(os.Symlink(policy, filepath.Join(repo, linkedNested))).To(Succeed())
			gitIn(repo, "add", linkedNested)
			commit := exec.Command("git", "commit", "-q", "-m", "link the policy")
			commit.Dir = repo
			commit.Env = commitEnv
			output, err := commit.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "git commit: %s", output)
			Expect(os.RemoveAll(filepath.Join(repo, "cfg"))).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, "cfg"), []byte("notes"), 0o644)).To(Succeed())
		})

		It("does not say to restore the policy from git, which would recreate a link the scan refuses", func() {
			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", linkedNested, "--format=json")

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(string(stderr)).To(ContainSubstring("a parent path component is not a directory"))
			Expect(string(stderr)).To(ContainSubstring("symbolic link"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("restore the policy from git"), "stderr:\n%s", stderr)
			expectNoRestoreNoAuthoring(stderr)
		})
	})
})
