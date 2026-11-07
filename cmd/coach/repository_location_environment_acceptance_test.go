package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// gitWith runs git in dir with extra environment settings and returns what it prints.
func gitWith(dir string, extra []string, args ...string) string {
	command := exec.Command("git", args...)
	command.Dir = dir
	command.Env = append(append([]string{}, commitEnv...), extra...)
	output, err := command.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git %s: %s", strings.Join(args, " "), output)
	return strings.TrimSpace(string(output))
}

// scanRevision runs a baseline scan in dir with the extra environment and returns the revision it reports on.
func scanRevision(dir string, extra []string) string {
	stdout, stderr, exitCode := runCoachBinary(commandPath, dir, append(os.Environ(), extra...), "codesignal", "--baseline", "--format=json", "--no-interactive")
	ExpectWithOffset(1, exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
	return decodeCoachReport(stdout).Scope.Revision
}

// The variables that say where a repository, its work tree and its objects are belong to whatever started Coach.
// A setup that keeps the git directory elsewhere, and a server-side hook whose objects arrive through a
// quarantine directory and the repository's own, are found only through them. v0.11.0 read the repository they
// name, and a scan that ignored them said it was not inside a work tree, or read objects that were not there.
var _ = Describe("coach codesignal: the variables that locate the repository are honoured as the caller set them", func() {
	DescribeTable("scans a work tree whose git directory is elsewhere and which has no .git of its own",
		func(relative bool) {
			root := GinkgoT().TempDir()
			gitWith(root, nil, "init", "-q", "--separate-git-dir=store.git", "wt")
			worktree, store := filepath.Join(root, "wt"), filepath.Join(root, "store.git")
			head := commitFile(worktree, "a.go", minimalGoSource)
			Expect(os.Remove(filepath.Join(worktree, ".git"))).To(Succeed())
			dir, gitDir, workTree := worktree, store, worktree
			if relative {
				gitDir, workTree = "../store.git", "."
			}

			revision := scanRevision(dir, []string{"GIT_DIR=" + gitDir, "GIT_WORK_TREE=" + workTree})

			Expect(revision).To(Equal(head))
		},
		Entry("named by absolute paths, from inside the work tree", false),
		Entry("named by paths relative to the work tree it was started in", true),
	)

	// The repository's own object directory is emptied into elsewhere, so the commit can be read only through
	// the variable.
	moveObjectsAway := func(repo string) string {
		objects := filepath.Join(repo, ".git", "objects")
		elsewhere := filepath.Join(GinkgoT().TempDir(), "objects")
		Expect(os.Rename(objects, elsewhere)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(objects, "pack"), 0o755)).To(Succeed())
		Expect(os.MkdirAll(filepath.Join(objects, "info"), 0o755)).To(Succeed())
		return elsewhere
	}

	DescribeTable("reads objects that only the variables lead to",
		func(variable string) {
			repo := commitScannableGoRepo()
			head := strings.TrimSpace(gitOutputIn(repo, "rev-parse", "HEAD"))
			elsewhere := moveObjectsAway(repo)

			Expect(scanRevision(repo, []string{variable + "=" + elsewhere})).To(Equal(head))
		},
		Entry("GIT_OBJECT_DIRECTORY", "GIT_OBJECT_DIRECTORY"),
		Entry("GIT_ALTERNATE_OBJECT_DIRECTORIES", "GIT_ALTERNATE_OBJECT_DIRECTORIES"),
	)

	It("reads a quarantine: the new commit only in GIT_OBJECT_DIRECTORY, the history only through the alternates", func() {
		repo := commitScannableGoRepo()
		quarantine := GinkgoT().TempDir()
		environment := []string{"GIT_OBJECT_DIRECTORY=" + quarantine, "GIT_ALTERNATE_OBJECT_DIRECTORIES=" + filepath.Join(repo, ".git", "objects")}
		Expect(os.WriteFile(filepath.Join(repo, "a.go"), []byte(minimalGoSource+"// received\n"), 0o644)).To(Succeed())
		gitWith(repo, environment, "commit", "-q", "-a", "-m", "received")
		received := gitWith(repo, environment, "rev-parse", "HEAD")
		Expect(exec.Command("git", "-C", repo, "cat-file", "-e", received).Run()).NotTo(Succeed(), "the new commit must be only in the quarantine")

		Expect(scanRevision(repo, environment)).To(Equal(received))
	})
})
