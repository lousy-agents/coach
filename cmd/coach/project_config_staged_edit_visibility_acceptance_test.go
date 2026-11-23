package main

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: a staged edit to a committed explicit policy leaves dirty_worktree, because the check looked for the file on disk", func() {
	const (
		nested       = "cfg/p.json"
		stagedPolicy = `{"schema_version":"1","roots":["staged"]}`
	)
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		commitFile(repo, nested, singleRootPolicyJSON)
		Expect(os.WriteFile(filepath.Join(repo, nested), []byte(stagedPolicy), 0o644)).To(Succeed())
		gitIn(repo, "add", nested)
	})

	When("a file stands where the parent directory was", func() {
		BeforeEach(func() {
			Expect(os.RemoveAll(filepath.Join(repo, "cfg"))).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, "cfg"), []byte("notes"), 0o644)).To(Succeed())
		})

		It("leaves the policy path out of dirty_worktree, because the check looked for the file on disk", func() {
			doc := expectCheckProject(repo, "--project-config", nested)

			Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
			Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement(nested))
		})

		It("names the staged edit and does not say to restore the policy from git", func() {
			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", nested, "--format=json")

			Expect(exitCode).To(Equal(2), "stdout=%s stderr=%s", stdout, stderr)
			Expect(string(stderr)).To(ContainSubstring("project_config_missing"))
			Expect(string(stderr)).To(ContainSubstring("a parent path component is not a directory"))
			Expect(string(stderr)).To(ContainSubstring("staged edit"), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("restore the policy from git"), "that advice discards the staged edit:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("to drop it and bring back the committed copy instead, move the file that blocks the path, then restore it with git restore --source="), "a restore from the commit is offered only as the way to drop the edit, after the file is moved:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("move the file that blocks the path, then restore it with git restore -- ':(top,literal)"+nested+"'"), "the staged edit is kept by a restore from the index once the file is moved:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring(suggestCommand), "stderr:\n%s", stderr)
			Expect(gitIn(repo, "show", ":"+nested)).To(Equal(stagedPolicy), "the staged edit is untouched")
		})
	})

	When("a symbolic link stands at the path", func() {
		BeforeEach(func() {
			full := filepath.Join(repo, nested)
			Expect(os.Remove(full)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, "cfg", "real.json"), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())
			Expect(os.Symlink("real.json", full)).To(Succeed())
		})

		It("leaves the policy path out of dirty_worktree, because the check looked for the file on disk", func() {
			doc := expectCheckProject(repo, "--project-config", nested)

			Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
			Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement(nested))
		})
	})
})

var _ = Describe("coach codesignal --check-project: a staged policy that was never committed leaves dirty_worktree, because the check looked for the file on disk", func() {
	const (
		nested       = "cfg/p.json"
		stagedPolicy = `{"schema_version":"1","roots":["staged"]}`
	)
	var repo string

	BeforeEach(func() {
		repo = commitScannableGoRepo()
		Expect(os.MkdirAll(filepath.Join(repo, "cfg"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, nested), []byte(stagedPolicy), 0o644)).To(Succeed())
		gitIn(repo, "add", nested)
	})

	It("leaves the path out of dirty_worktree when the file was removed from the working tree", func() {
		Expect(os.Remove(filepath.Join(repo, nested))).To(Succeed())

		doc := expectCheckProject(repo, "--project-config", nested)

		Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
		Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement(nested))
	})

	It("leaves the path out of dirty_worktree when a file stands where the parent directory was", func() {
		Expect(os.RemoveAll(filepath.Join(repo, "cfg"))).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, "cfg"), []byte("notes"), 0o644)).To(Succeed())

		doc := expectCheckProject(repo, "--project-config", nested)

		Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
		Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement(nested))
	})

	It("leaves the path out of dirty_worktree when a symbolic link stands at the path", func() {
		full := filepath.Join(repo, nested)
		Expect(os.Remove(full)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, "cfg", "real.json"), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())
		Expect(os.Symlink("real.json", full)).To(Succeed())

		doc := expectCheckProject(repo, "--project-config", nested)

		Expect(doc.Checks.Policy.Code).To(Equal("policy_invalid"))
		Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement(nested))
	})
})

var _ = Describe("coach codesignal --check-project: a regular file staged over a policy committed as a symbolic link leaves dirty_worktree", func() {
	const linked = "cfg/linked.json"

	It("leaves the path out of dirty_worktree when the staged file was removed from the working tree", func() {
		repo := commitScannableGoRepo()
		Expect(os.MkdirAll(filepath.Join(repo, "cfg"), 0o755)).To(Succeed())
		full := filepath.Join(repo, linked)
		Expect(os.Symlink("real.json", full)).To(Succeed())
		gitIn(repo, "add", linked)
		commit := exec.Command("git", "commit", "-q", "-m", "link the policy")
		commit.Dir = repo
		commit.Env = commitEnv
		output, err := commit.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), "git commit: %s", output)
		Expect(os.Remove(full)).To(Succeed())
		Expect(os.WriteFile(full, []byte(singleRootPolicyJSON), 0o644)).To(Succeed())
		gitIn(repo, "add", linked)
		Expect(os.Remove(full)).To(Succeed())

		doc := expectCheckProject(repo, "--project-config", linked)

		Expect(doc.Checks.Policy.Code).To(Equal("policy_missing"))
		Expect(doc.DirtyWorktree.Paths).NotTo(ContainElement(linked))
	})
})
