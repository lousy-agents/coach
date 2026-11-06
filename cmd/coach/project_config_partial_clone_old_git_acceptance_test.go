package main

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const unreachableRemote = "ssh://example.invalid/repo.git"

var (
	scanArgs         = []string{"codesignal", "--baseline", "--project-config", "cfg/p.json", "--format=json", "--no-interactive"}
	checkProjectArgs = []string{"codesignal", "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "cfg/p.json", "--format", "json"}
)

// oldGitFetchingNothing is the environment of a git that says it is 2.43.0, and so cannot be told not to
// fetch, whose ssh program leaves a marker when git runs it.
func oldGitFetchingNothing() (env []string, marker string) {
	sshCommand, marker := sshProgramThatLeavesAMarker()
	return append(environmentWithGitReporting("2.43.0"), "GIT_SSH_COMMAND="+sshCommand), marker
}

func expectRefusalOnOldGit(stderr []byte, words ...string) {
	for _, word := range append([]string{"partial clone", "git 2.43.0", "git 2.44 or later"}, words...) {
		ExpectWithOffset(1, string(stderr)).To(ContainSubstring(word), "stderr:\n%s", stderr)
	}
}

// An older git ignores the instruction not to fetch, so a partial clone is read only when every object of
// the revisions the scan reads is already here: then there is nothing for git to fetch.
var _ = Describe("coach codesignal: an older git reads a partial clone that lacks nothing the scan reads", func() {
	DescribeTable("refuses a partial clone that lacks objects of HEAD, says how to get them, and fetches nothing",
		func(args []string) {
			env, marker := oldGitFetchingNothing()
			clone := commitPartialClone(func(string) string { return unreachableRemote })
			before := packFilesIn(clone)

			stdout, stderr, exitCode := runCoachBinary(commandPath, clone, env, args...)

			Expect(exitCode).To(Equal(1), "stdout: %s\nstderr: %s", stdout, stderr)
			expectRefusalOnOldGit(stderr, "HEAD has 2 objects that are not in it", "git rev-list --objects --missing=print --no-walk HEAD")
			Expect(marker).NotTo(BeAnExistingFile(), "git ran the ssh program")
			Expect(packFilesIn(clone)).To(Equal(before), "the run wrote objects into the repository")
		},
		Entry("a scan", scanArgs),
		Entry("--check-project", checkProjectArgs),
	)

	It("reads a partial clone whose objects of HEAD are all here", func() {
		env, marker := oldGitFetchingNothing()
		clone := completePartialCloneOf(commitScannableGoRepo(), func(string) string { return unreachableRemote })

		stdout, stderr, exitCode := runCoachBinary(commandPath, clone, env, "codesignal", "--baseline", "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(string(stderr)).NotTo(ContainSubstring("partial clone"), "stderr:\n%s", stderr)
		Expect(marker).NotTo(BeAnExistingFile(), "git ran the ssh program")
	})

	It("recognises a promisor setting with no value, which git reads as true", func() {
		env, marker := oldGitFetchingNothing()
		clone := commitPartialClone(func(string) string { return unreachableRemote })
		setPromisorWithoutAValue(clone)

		stdout, stderr, exitCode := runCoachBinary(commandPath, clone, env, scanArgs...)

		Expect(exitCode).To(Equal(1), "stdout: %s\nstderr: %s", stdout, stderr)
		expectRefusalOnOldGit(stderr, "HEAD has 2 objects that are not in it")
		Expect(marker).NotTo(BeAnExistingFile(), "git ran the ssh program")
	})

	It("refuses when it cannot tell which objects the scan reads, rather than reading on", func() {
		env, marker := oldGitFetchingNothing()
		clone := commitPartialClone(func(string) string { return unreachableRemote })

		stdout, stderr, exitCode := runCoachBinary(commandPath, clone, env, "codesignal", "--base", "no-such-ref", "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(1), "stdout: %s\nstderr: %s", stdout, stderr)
		expectRefusalOnOldGit(stderr, "could not tell whether this partial clone has the objects Coach reads")
		Expect(marker).NotTo(BeAnExistingFile(), "git ran the ssh program")
	})
})

// A diff reads the merge base as well as HEAD, so the merge base's objects count too.
var _ = Describe("coach codesignal: an older git reads a diff in a partial clone only when the merge base is here too", func() {
	var clone string

	BeforeEach(func() {
		source := commitScannableGoRepo()
		commitFile(source, "a.go", minimalGoSource+"// changed\n")
		clone = completePartialCloneOf(source, func(string) string { return unreachableRemote })
	})

	It("refuses a diff whose merge base lacks the objects that HEAD changed", func() {
		env, marker := oldGitFetchingNothing()
		before := packFilesIn(clone)

		stdout, stderr, exitCode := runCoachBinary(commandPath, clone, env, "codesignal", "--base", "HEAD~1", "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(1), "stdout: %s\nstderr: %s", stdout, stderr)
		expectRefusalOnOldGit(stderr, "the merge base of HEAD~1 and HEAD has 1 object that is not in it")
		Expect(marker).NotTo(BeAnExistingFile(), "git ran the ssh program")
		Expect(packFilesIn(clone)).To(Equal(before), "the run wrote objects into the repository")
	})

	It("reads a baseline of the same clone, which needs only HEAD", func() {
		env, _ := oldGitFetchingNothing()

		stdout, stderr, exitCode := runCoachBinary(commandPath, clone, env, "codesignal", "--baseline", "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
	})
})

func setPromisorWithoutAValue(clone string) {
	config := filepath.Join(clone, ".git", "config")
	contents, err := os.ReadFile(config)
	ExpectWithOffset(1, err).NotTo(HaveOccurred())
	valueless := strings.Replace(string(contents), "\tpromisor = true\n", "\tpromisor\n", 1)
	ExpectWithOffset(1, valueless).NotTo(Equal(string(contents)), "the clone's configuration did not hold the setting this spec rewrites")
	ExpectWithOffset(1, os.WriteFile(config, []byte(valueless), 0o644)).To(Succeed())
}

var _ = Describe("coach codesignal: an older git still scans a repository that is not a partial clone", func() {
	DescribeTable("whatever its remotes say about promising",
		func(arrange func(repo string)) {
			repo := commitOrdinaryClone()
			arrange(repo)

			stdout, stderr, exitCode := runCoachBinary(commandPath, repo, environmentWithGitReporting("2.43.0"), "codesignal", "--baseline", "--format=json", "--no-interactive")

			Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("partial clone"), "stderr:\n%s", stderr)
		},
		Entry("a clone with an ordinary remote and no promisor setting", func(string) {}),
		Entry("a remote whose promisor setting is false", func(repo string) {
			gitIn(repo, "config", "remote.origin.promisor", "false")
		}),
		Entry("a promisor remote beside one that is not, the promisor one set to false", func(repo string) {
			gitIn(repo, "remote", "add", "upstream", "https://example.invalid/upstream.git")
			gitIn(repo, "config", "remote.upstream.promisor", "false")
		}),
		Entry("a filter recorded for a remote that is not a promisor", func(repo string) {
			gitIn(repo, "config", "remote.origin.partialclonefilter", "blob:none")
		}),
		Entry("an empty extensions.partialclone", func(repo string) {
			gitIn(repo, "config", "extensions.partialclone", "")
		}),
		Entry("a repository with no remote at all", func(repo string) {
			gitIn(repo, "remote", "remove", "origin")
		}),
	)
})
