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

	It("runs no hook, external diff, textconv driver, pager, askpass or ssh program", func() {
		marker := filepath.Join(GinkgoT().TempDir(), "ran")
		program := script(marker)
		repo := commitExplicitPolicyRepo()
		Expect(os.WriteFile(filepath.Join(repo, ".gitattributes"), []byte("*.go diff=evil\n"), 0o644)).To(Succeed())
		gitIn(repo, "add", ".gitattributes")
		attrs := exec.Command("git", "commit", "-q", "-m", "attrs")
		attrs.Dir, attrs.Env = repo, commitEnv
		Expect(attrs.Run()).To(Succeed())
		// installed after the setup commit, which would otherwise run them itself
		hooks := filepath.Join(repo, ".git", "hooks")
		for _, hook := range []string{"post-checkout", "reference-transaction", "post-index-change", "post-merge"} {
			Expect(os.WriteFile(filepath.Join(hooks, hook), []byte("#!/bin/sh\ntouch '"+marker+"'\n"), 0o755)).To(Succeed())
		}
		gitIn(repo, "config", "diff.evil.textconv", "touch '"+marker+"'; cat")
		gitIn(repo, "config", "diff.external", program)
		Expect(os.WriteFile(filepath.Join(repo, "a.go"), []byte(minimalGoSource+"// changed\n"), 0o644)).To(Succeed())
		env := append(os.Environ(), "GIT_EXTERNAL_DIFF="+program, "GIT_PAGER="+program, "PAGER="+program, "GIT_ASKPASS="+program, "GIT_SSH_COMMAND="+program, "GIT_EDITOR="+program)

		for _, args := range [][]string{
			{"codesignal", "--base", "HEAD", "--project-config", explicitPolicyPath, "--format=json"},
			{"codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json"},
		} {
			_, stderr, exitCode := runCoachBinary(commandPath, repo, env, args...)
			Expect(exitCode).To(BeNumerically("<=", 1), "stderr: %s", stderr)
		}

		Expect(marker).NotTo(BeAnExistingFile(), "git ran a program the repository's configuration or the environment names")
	})

	It("fails closed, with no report, when GIT_WORK_TREE and GIT_INDEX_FILE point elsewhere, or GIT_DIR is a bare repository seen from a worktree subdirectory", func() {
		repo := commitExplicitPolicyRepo()
		stdout, _, exitCode := runCoachBinary(commandPath, repo, append(os.Environ(), "GIT_WORK_TREE="+GinkgoT().TempDir(), "GIT_INDEX_FILE=/nonexistent/index"), "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(exitCode).NotTo(Equal(0))
		Expect(stdout).To(BeEmpty())

		bare := repo + ".bare"
		Expect(exec.Command("git", "clone", "-q", "--bare", repo, bare).Run()).To(Succeed())
		DeferCleanup(func() { _ = os.RemoveAll(bare) })
		subdirectory := filepath.Join(repo, "policies")
		stdout, _, exitCode = runCoachBinary(commandPath, subdirectory, append(os.Environ(), "GIT_DIR="+bare), "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(exitCode).NotTo(Equal(0))
		Expect(stdout).To(BeEmpty())
	})

	It("fails closed, with no report, when GIT_DIR or a .git file points at a different repository", func() {
		repo := commitExplicitPolicyRepo()
		other := commitScannableGoRepo()

		_, _, viaEnv := runCoachBinary(commandPath, repo, append(os.Environ(), "GIT_DIR="+filepath.Join(other, ".git")), "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(viaEnv).NotTo(Equal(0), "a scan read a policy from the working tree of one repository and the history of another")

		Expect(os.RemoveAll(filepath.Join(repo, ".git"))).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, ".git"), []byte("gitdir: "+filepath.Join(other, ".git")+"\n"), 0o644)).To(Succeed())
		stdout, _, viaFile := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(viaFile).NotTo(Equal(0))
		Expect(stdout).To(BeEmpty())
	})
})
