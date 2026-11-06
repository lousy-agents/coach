package main

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach codesignal: a partial clone that lacks the policy's blob is read, never fetched into", func() {
	var clone, marker string
	var env []string

	DescribeTable("runs no program and writes no object, and declines cleanly",
		func(remoteURL func(bare string) string, scan bool) {
			skipUnlessGitCanRefuseLazyFetch()
			sshCommand, sshMarker := sshProgramThatLeavesAMarker()
			marker = sshMarker
			env = append(os.Environ(), "GIT_SSH_COMMAND="+sshCommand)
			clone = commitPartialClone(remoteURL)
			before := packFilesIn(clone)

			args := []string{"codesignal", "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "cfg/p.json", "--format", "json"}
			want := "could not check whether it is committed at the analyzed revision"
			if scan {
				args = []string{"codesignal", "--baseline", "--project-config", "cfg/p.json", "--format=json", "--no-interactive"}
				want = "reading source snapshot"
			}
			stdout, stderr, exitCode := runCoachBinary(commandPath, clone, env, args...)

			Expect(exitCode).To(Equal(1), "an object that is not local is an operational failure:\nstdout: %s\nstderr: %s", stdout, stderr)
			Expect(string(stderr)).To(ContainSubstring(want), "stderr:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("is not committed"), "the policy is committed; its object is what is not here:\n%s", stderr)
			Expect(string(stderr)).NotTo(ContainSubstring("git restore"), "no restore is offered for a blob that cannot be checked:\n%s", stderr)
			Expect(marker).NotTo(BeAnExistingFile(), "git ran the ssh program to fetch the object")
			Expect(packFilesIn(clone)).To(Equal(before), "the scan wrote objects into the repository")
		},
		Entry("--check-project, remote that would run a program", func(string) string { return "ssh://example.invalid/repo.git" }, false),
		Entry("--check-project, remote that would answer", func(bare string) string { return "file://" + bare }, false),
		Entry("a scan, remote that would run a program", func(string) string { return "ssh://example.invalid/repo.git" }, true),
		Entry("a scan, remote that would answer", func(bare string) string { return "file://" + bare }, true),
	)

})

var _ = Describe("coach codesignal: a git that cannot be told not to fetch is refused in a partial clone", func() {
	DescribeTable("names the version and the version that is needed, and fetches nothing",
		func(args ...string) {
			sshCommand, marker := sshProgramThatLeavesAMarker()
			clone := commitPartialClone(func(string) string { return "ssh://example.invalid/repo.git" })
			before := packFilesIn(clone)
			env := append(environmentWithGitReporting("2.43.0"), "GIT_SSH_COMMAND="+sshCommand)

			stdout, stderr, exitCode := runCoachBinary(commandPath, clone, env, args...)

			Expect(exitCode).To(Equal(1), "stdout: %s\nstderr: %s", stdout, stderr)
			Expect(string(stderr)).To(ContainSubstring("partial clone"), "stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("git 2.43.0"), "stderr:\n%s", stderr)
			Expect(string(stderr)).To(ContainSubstring("git 2.44 or later"), "stderr:\n%s", stderr)
			Expect(marker).NotTo(BeAnExistingFile(), "git ran the ssh program")
			Expect(packFilesIn(clone)).To(Equal(before), "the run wrote objects into the repository")
		},
		Entry("a scan", "codesignal", "--baseline", "--project-config", "cfg/p.json", "--format=json", "--no-interactive"),
		Entry("--check-project", "codesignal", "--baseline", "--check-project", "--project-language", "typescript", "--project-config", "cfg/p.json", "--format", "json"),
	)

	DescribeTable("still scans a repository that is not a partial clone, whatever its remotes say about promising",
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
