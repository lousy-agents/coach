package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// hookRun is what Coach wrote when a pre-commit hook ran it, and the revision HEAD was at when it ran.
type hookRun struct {
	stdout, stderr []byte
	exitCode       string
}

// commitRunningCoachInHook installs a pre-commit hook that runs Coach with args and records what it did,
// then makes the commit in directory. git gives a hook its own GIT_DIR, GIT_INDEX_FILE (a temporary index
// for a partial commit) and, in a linked worktree, the worktree's own git directory.
func commitRunningCoachInHook(directory string, args []string, commit ...string) hookRun {
	out := GinkgoT().TempDir()
	hook := strings.TrimSpace(gitOutputIn(directory, "rev-parse", "--path-format=absolute", "--git-path", "hooks/pre-commit"))
	ExpectWithOffset(1, os.MkdirAll(filepath.Dir(hook), 0o755)).To(Succeed())
	script := "#!/bin/sh\n\"$COACH\" " + strings.Join(args, " ") + " > \"$HOOK_OUT/stdout\" 2> \"$HOOK_OUT/stderr\"\necho $? > \"$HOOK_OUT/exit\"\nexit 0\n"
	ExpectWithOffset(1, os.WriteFile(hook, []byte(script), 0o755)).To(Succeed())

	command := exec.Command("git", append([]string{"commit", "-q"}, commit...)...)
	command.Dir = directory
	command.Env = append(commitEnv, "COACH="+commandPath, "HOOK_OUT="+out)
	output, err := command.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git commit: %s", output)

	read := func(name string) []byte {
		contents, readErr := os.ReadFile(filepath.Join(out, name))
		ExpectWithOffset(2, readErr).NotTo(HaveOccurred(), "the hook did not run Coach")
		return contents
	}
	return hookRun{stdout: read("stdout"), stderr: read("stderr"), exitCode: strings.TrimSpace(string(read("exit")))}
}

// git sets GIT_DIR, GIT_INDEX_FILE and GIT_WORK_TREE for a hook, and Coach removes them: it finds the
// repository from the directory it is started in, which for a hook is the root of the work tree, and for a
// linked worktree is that worktree and not the repository it was added from. The index it reads is the real one.
var _ = Describe("coach codesignal run by a pre-commit hook reads the repository it was started in", func() {
	scanArgs := []string{"codesignal", "--baseline", "--format=json", "--no-interactive"}

	It("scans HEAD of the repository, in a commit that updates the index with -a", func() {
		repo := commitScannableGoRepo()
		before := strings.TrimSpace(gitOutputIn(repo, "rev-parse", "HEAD"))
		Expect(os.WriteFile(filepath.Join(repo, "a.go"), []byte(minimalGoSource+"// edit\n"), 0o644)).To(Succeed())

		run := commitRunningCoachInHook(repo, scanArgs, "-a", "-m", "edit")

		Expect(run.exitCode).To(Equal("0"), "stderr: %s", run.stderr)
		Expect(decodeCoachReport(run.stdout).Scope.Revision).To(Equal(before), "the hook ran before the commit, so HEAD is the commit it builds on")
	})

	It("scans the linked worktree it was started in, not the repository the worktree was added from", func() {
		repo := commitScannableGoRepo()
		worktree := filepath.Join(GinkgoT().TempDir(), "linked")
		gitOutputIn(repo, "worktree", "add", "-q", "-b", "linked-branch", worktree)
		Expect(os.WriteFile(filepath.Join(worktree, "a.go"), []byte(minimalGoSource+"// in the worktree\n"), 0o644)).To(Succeed())
		gitOutputIn(worktree, "commit", "-q", "-a", "-m", "move the worktree ahead", "--no-verify")
		Expect(os.WriteFile(filepath.Join(worktree, "a.go"), []byte(minimalGoSource+"// and again\n"), 0o644)).To(Succeed())
		worktreeHead := strings.TrimSpace(gitOutputIn(worktree, "rev-parse", "HEAD"))
		Expect(worktreeHead).NotTo(Equal(strings.TrimSpace(gitOutputIn(repo, "rev-parse", "HEAD"))))

		run := commitRunningCoachInHook(worktree, scanArgs, "-a", "-m", "again")

		Expect(run.exitCode).To(Equal("0"), "stderr: %s", run.stderr)
		Expect(decodeCoachReport(run.stdout).Scope.Revision).To(Equal(worktreeHead))
	})

	It("reads the real index, so a staged edit that a partial commit leaves out is still disclosed", func() {
		const policy = "cfg/p.json"
		repo := commitScannableGoRepo()
		commitFile(repo, policy, singleRootPolicyJSON)
		Expect(os.WriteFile(filepath.Join(repo, policy), []byte(`{"schema_version":"1","roots":["staged"]}`), 0o644)).To(Succeed())
		gitOutputIn(repo, "add", policy)
		Expect(os.Remove(filepath.Join(repo, policy))).To(Succeed())
		Expect(os.WriteFile(filepath.Join(repo, "a.go"), []byte(minimalGoSource+"// edit\n"), 0o644)).To(Succeed())

		run := commitRunningCoachInHook(repo, []string{"codesignal", "--baseline", "--project-config", policy, "--format=json", "--no-interactive"}, "-o", "a.go", "-m", "edit")

		Expect(run.exitCode).To(Equal("2"), "stdout: %s\nstderr: %s", run.stdout, run.stderr)
		Expect(string(run.stderr)).To(ContainSubstring("the index holds a staged edit to it that a restore would discard"), "stderr:\n%s", run.stderr)
		Expect(string(run.stderr)).NotTo(ContainSubstring("git restore"), "that restore would discard the staged edit:\n%s", run.stderr)
	})
})
