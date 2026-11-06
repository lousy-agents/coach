package main

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: a hostile git configuration or environment executes nothing", func() {
	script := func(marker string) string {
		path := filepath.Join(GinkgoT().TempDir(), "run.sh")
		Expect(os.WriteFile(path, []byte("#!/bin/sh\ntouch '"+marker+"'\nprintf '\\0'\n"), 0o755)).To(Succeed())
		return path
	}

	It("runs no core.fsmonitor program, from the repository config or from GIT_CONFIG_COUNT", func() {
		marker := filepath.Join(GinkgoT().TempDir(), "ran")
		program := script(marker)
		repo := commitExplicitPolicyRepo()
		gitIn(repo, "config", "core.fsmonitor", program)
		env := append(os.Environ(), "GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=core.fsmonitor", "GIT_CONFIG_VALUE_0="+program)

		_, stderr, exitCode := runCoachBinary(commandPath, repo, env, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(os.Remove(filepath.Join(repo, explicitPolicyPath))).To(Succeed())
		refusedViaEnv := func() {
			_, _, code := runCoachBinary(commandPath, repo, env, "codesignal", "--baseline", "--project-config", explicitPolicyPath)
			Expect(code).To(Equal(2))
		}
		refusedViaEnv()

		Expect(marker).NotTo(BeAnExistingFile(), "git ran the core.fsmonitor program the repository's configuration names")
	})

	// Each program has its own marker, so a failure names the one that ran. The textconv driver and the
	// external diff program are not here: git runs them on a patch, and a scan prints one only for a file
	// that changed between the base and HEAD (hostile_diff_driver_acceptance_test.go).
	DescribeTable("runs no program the repository or the environment names, whichever channel names it",
		func(arrange func(repo, program string, installHook func(name string)) []string, programChannel string) {
			marker := filepath.Join(GinkgoT().TempDir(), "ran")
			program := script(marker)
			repo := commitExplicitPolicyRepo()
			// installed after the setup commit, which would otherwise run them itself
			installHook := func(name string) {
				hook := filepath.Join(repo, ".git", "hooks", name)
				Expect(os.WriteFile(hook, []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o755)).To(Succeed())
			}
			// a changed file in the worktree, so a read that is allowed to refresh the index has a reason to
			Expect(os.WriteFile(filepath.Join(repo, "a.go"), []byte(minimalGoSource+"// changed\n"), 0o644)).To(Succeed())
			env := append(os.Environ(), arrange(repo, program, installHook)...)

			for _, args := range [][]string{
				{"codesignal", "--base", "HEAD", "--project-config", explicitPolicyPath, "--format=json"},
				{"codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json"},
			} {
				_, stderr, exitCode := runCoachBinary(commandPath, repo, env, args...)
				Expect(exitCode).To(BeNumerically("<=", 1), "stderr: %s", stderr)
			}

			Expect(marker).NotTo(BeAnExistingFile(), "git ran a program named by "+programChannel)
		},
		Entry("the post-index-change hook, which git runs when a read rewrites the index", func(_, _ string, installHook func(string)) []string {
			installHook("post-index-change")
			return nil
		}, "the post-index-change hook"),
		Entry("the post-checkout hook", func(_, _ string, installHook func(string)) []string {
			installHook("post-checkout")
			return nil
		}, "the post-checkout hook"),
		Entry("the reference-transaction hook", func(_, _ string, installHook func(string)) []string {
			installHook("reference-transaction")
			return nil
		}, "the reference-transaction hook"),
		Entry("the post-merge hook", func(_, _ string, installHook func(string)) []string {
			installHook("post-merge")
			return nil
		}, "the post-merge hook"),
		// No read Coach runs starts a pager, asks for a credential, opens an ssh connection or opens an
		// editor, so these entries pass for the right reason today. They stand guard for a read that
		// would, and each names its variable when it fails.
		Entry("GIT_PAGER", func(_, program string, _ func(string)) []string { return []string{"GIT_PAGER=" + program} }, "GIT_PAGER"),
		Entry("PAGER", func(_, program string, _ func(string)) []string { return []string{"PAGER=" + program} }, "PAGER"),
		Entry("GIT_ASKPASS", func(_, program string, _ func(string)) []string { return []string{"GIT_ASKPASS=" + program} }, "GIT_ASKPASS"),
		Entry("GIT_SSH_COMMAND", func(_, program string, _ func(string)) []string { return []string{"GIT_SSH_COMMAND=" + program} }, "GIT_SSH_COMMAND"),
		Entry("GIT_EDITOR", func(_, program string, _ func(string)) []string { return []string{"GIT_EDITOR=" + program} }, "GIT_EDITOR"),
	)

	It("fails closed, with no report, when GIT_WORK_TREE and GIT_INDEX_FILE point elsewhere, or GIT_DIR is a bare repository seen from a worktree subdirectory", func() {
		repo := commitExplicitPolicyRepo()
		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, append(os.Environ(), "GIT_WORK_TREE="+GinkgoT().TempDir(), "GIT_INDEX_FILE=/nonexistent/index"), "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(exitCode).To(Equal(1), "an operational failure, not a crash and not a report: stderr: %s", stderr)
		Expect(string(stderr)).To(ContainSubstring("is not inside a Git worktree"))
		Expect(stdout).To(BeEmpty())

		bare := repo + ".bare"
		Expect(exec.Command("git", "clone", "-q", "--bare", repo, bare).Run()).To(Succeed())
		DeferCleanup(func() { _ = os.RemoveAll(bare) })
		subdirectory := filepath.Join(repo, "policies")
		stdout, stderr, exitCode = runCoachBinary(commandPath, subdirectory, append(os.Environ(), "GIT_DIR="+bare), "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(exitCode).To(Equal(1), "an operational failure, not a crash and not a report: stderr: %s", stderr)
		Expect(string(stderr)).To(ContainSubstring("is not inside a Git worktree"))
		Expect(stdout).To(BeEmpty())
	})

	It("fails closed, with no report, when GIT_DIR or a .git file points at a different repository", func() {
		repo := commitExplicitPolicyRepo()
		other := commitScannableGoRepo()

		stdout, stderr, viaEnv := runCoachBinary(commandPath, repo, append(os.Environ(), "GIT_DIR="+filepath.Join(other, ".git")), "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(viaEnv).To(Equal(2), "a scan read a policy from the working tree of one repository and the history of another: stderr: %s", stderr)
		Expect(string(stderr)).To(ContainSubstring("exists in the worktree but is not committed"))
		Expect(stdout).To(BeEmpty())

		Expect(os.RemoveAll(filepath.Join(repo, ".git"))).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: "+filepath.Join(other, ".git")+"\n"), 0o644)).To(Succeed())
		stdout, stderr, viaFile := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(viaFile).To(Equal(2), "stderr: %s", stderr)
		Expect(string(stderr)).To(ContainSubstring("exists in the worktree but is not committed"))
		Expect(stdout).To(BeEmpty())
	})
})
