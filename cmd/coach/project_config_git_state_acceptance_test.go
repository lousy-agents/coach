package main

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: git states around an explicit --project-config", func() {
	It("still scans when a stale .git/index.lock is present and the policy is intact", func() {
		repo := commitExplicitPolicyRepo()
		Expect(os.WriteFile(filepath.Join(repo, ".git", "index.lock"), nil, 0o644)).To(Succeed())

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(stdout).NotTo(BeEmpty())
	})

	It("counts an intent-to-add entry for a deleted policy as no entry: no restore command, authoring offered", func() {
		repo := commitScannableGoRepo()
		Expect(os.MkdirAll(filepath.Join(repo, "policies"), 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, explicitPolicyPath), []byte(singleRootPolicyJSON), 0o644)).To(Succeed())
		gitIn(repo, "add", "-N", "--", explicitPolicyPath)
		Expect(os.Remove(filepath.Join(repo, explicitPolicyPath))).To(Succeed())

		stderr := expectRefusedWith(repo, explicitPolicyPath, "project_config_missing", "was not found on disk")

		Expect(stderr).NotTo(ContainSubstring("git restore"), "an entry with no content has nothing to restore")
		Expect(stderr).To(ContainSubstring("--suggest-project-config"))
	})

	It("scans from a detached HEAD", func() {
		repo := commitExplicitPolicyRepo()
		gitIn(repo, "checkout", "-q", "--detach", "HEAD")

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")

		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(stdout).NotTo(BeEmpty())
	})

	It("scans a linked worktree, and a policy deleted there is restored there and nowhere else", func() {
		repo := commitExplicitPolicyRepo()
		linked := repo + "-linked"
		gitIn(repo, "worktree", "add", "-q", linked)
		DeferCleanup(func() { _ = os.RemoveAll(linked) })

		stdout, stderr, exitCode := runCoachBinary(commandPath, linked, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(stdout).NotTo(BeEmpty())

		Expect(os.Remove(filepath.Join(linked, explicitPolicyPath))).To(Succeed())
		command := restoreCommandFrom([]byte(refusedScan(linked, explicitPolicyPath)))
		Expect(command).NotTo(BeEmpty())
		output, restoreExit := runInShell(linked, command)

		Expect(restoreExit).To(Equal(0), "command=%s output=%s", command, output)
		Expect(filepath.Join(linked, explicitPolicyPath)).To(BeARegularFile())
		Expect(gitIn(repo, "status", "--porcelain")).To(BeEmpty(), "the main worktree was touched")
	})

	It("reports an operational failure, not a policy error, with no commits, outside a repository, in a bare repository and for an unknown --base", func() {
		unborn := GinkgoT().TempDir()
		gitIn(unborn, "init", "-q")
		bareSource := commitExplicitPolicyRepo()
		bare := bareSource + ".bare"
		Expect(exec.Command("git", "clone", "-q", "--bare", bareSource, bare).Run()).To(Succeed())
		DeferCleanup(func() { _ = os.RemoveAll(bare) })
		repo := commitExplicitPolicyRepo()

		expectOperationalFailure(unborn, "HEAD is not readable", "--baseline")
		expectOperationalFailure(GinkgoT().TempDir(), "is not inside a Git worktree", "--baseline")
		expectOperationalFailure(bare, "is not inside a Git worktree", "--baseline")
		expectOperationalFailure(repo, `--base "no-such-ref" cannot be resolved to a commit`, "--base", "no-such-ref")
		expectOperationalFailure(repo, "cannot be resolved to a commit", "--base", "--output=x")
	})

	It("refuses a policy path that is a submodule, and one that runs into a submodule's checkout", func() {
		repo := commitRepoWithSubmodule()

		expectRefusedWith(repo, "vendor/sub", "project_config_invalid", "committed as a submodule")
		expectRefusedWith(repo, "vendor/sub/absent.json", "project_config_missing", "was not found on disk")
	})

	It("scans and restores in a SHA-256 repository", func() {
		repo := GinkgoT().TempDir()
		if out, err := exec.Command("git", "-C", repo, "init", "-q", "--object-format=sha256").CombinedOutput(); err != nil {
			Skip("this git cannot create a SHA-256 repository: " + string(out))
		}
		for name, content := range map[string]string{"a.go": minimalGoSource, explicitPolicyPath: singleRootPolicyJSON} {
			Expect(os.MkdirAll(filepath.Dir(filepath.Join(repo, name)), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(repo, name), []byte(content), 0o644)).To(Succeed())
		}
		gitIn(repo, "add", "-A")
		commit := exec.Command("git", "commit", "-q", "-m", "init")
		commit.Dir, commit.Env = repo, commitEnv
		Expect(commit.Run()).To(Succeed())

		stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", explicitPolicyPath, "--format=json")
		Expect(exitCode).To(Equal(0), "stderr: %s", stderr)
		Expect(stdout).NotTo(BeEmpty())

		Expect(os.Remove(filepath.Join(repo, explicitPolicyPath))).To(Succeed())
		command := restoreCommandFrom([]byte(refusedScan(repo, explicitPolicyPath)))
		Expect(command).To(MatchRegexp(`--source=[0-9a-f]{64} `), "the revision is the full 64-hex object name")
		output, restoreExit := runInShell(repo, command)
		Expect(restoreExit).To(Equal(0), "command=%s output=%s", command, output)
		Expect(filepath.Join(repo, explicitPolicyPath)).To(BeARegularFile())
	})

	It("resolves either Unicode spelling of a committed name on a filesystem that ignores normalization", func() {
		composed := "caf\u00e9.json"
		decomposed := "cafe\u0301.json"
		probe := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(probe, composed), nil, 0o644)).To(Succeed())
		if _, err := os.Stat(filepath.Join(probe, decomposed)); err != nil {
			Skip("this filesystem keeps composed and decomposed spellings apart")
		}
		for _, spelling := range []struct{ committed, passed string }{{composed, decomposed}, {decomposed, composed}} {
			repo := commitScannableGoRepo()
			commitFile(repo, spelling.committed, singleRootPolicyJSON)

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, nil, "codesignal", "--baseline", "--project-config", spelling.passed, "--format=json")

			Expect(exitCode).To(Equal(0), "committed %q, passed %q: %s", spelling.committed, spelling.passed, stderr)
			Expect(stdout).NotTo(BeEmpty())
		}
	})
})

// expectOperationalFailure runs a scan that cannot start, and requires exit 1 (an operational failure,
// not a policy error), no report, and the reason on standard error.
func expectOperationalFailure(dir, reason string, selection ...string) {
	args := append([]string{"codesignal"}, selection...)
	args = append(args, "--project-config", explicitPolicyPath)
	stdout, stderr, exitCode := runCoachBinary(commandPath, dir, nil, args...)

	ExpectWithOffset(1, exitCode).To(Equal(1), string(stderr))
	ExpectWithOffset(1, stdout).To(BeEmpty())
	ExpectWithOffset(1, string(stderr)).To(ContainSubstring(reason))
}
