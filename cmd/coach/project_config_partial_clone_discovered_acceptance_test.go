package main

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const discoveredPolicyPath = ".lousy-agents/coach.json"

// discoveredPolicyPartialClone is a filtered, no-checkout clone whose canonical policy is committed and
// whose blob is not local, so the policy is found by discovery and not named by --project-config.
func discoveredPolicyPartialClone() string {
	source := commitScannableGoRepo()
	commitFile(source, discoveredPolicyPath, singleRootPolicyJSON)
	return partialCloneOf(source, func(string) string { return unreachableRemote })
}

var (
	discoveredScanArgs         = []string{"codesignal", "--baseline", "--format=json", "--no-interactive"}
	discoveredCheckProjectArgs = []string{"codesignal", "--baseline", "--check-project", "--project-language", "typescript", "--format", "json"}
)

// With no --project-config the policy is the one discovery finds, so the objects a read needs include the
// canonical policy's, and a read that cannot have them says so instead of going on without a policy.
var _ = Describe("coach codesignal: a discovered policy that a partial clone lacks is explained, not skipped", func() {
	It("fails --check-project naming the policy and the command that fetches it", func() {
		skipUnlessGitCanRefuseLazyFetch()
		sshCommand, marker := sshProgramThatLeavesAMarker()
		clone := discoveredPolicyPartialClone()
		revision := strings.TrimSpace(gitOutputIn(clone, "rev-parse", "HEAD"))
		before := packFilesIn(clone)

		stdout, stderr, exitCode := runCoachBinary(commandPath, clone, append(os.Environ(), "GIT_SSH_COMMAND="+sshCommand), discoveredCheckProjectArgs...)

		Expect(exitCode).To(Equal(1), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring(discoveredPolicyPath), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("Coach does not fetch objects"), "stderr:\n%s", stderr)
		Expect(string(stderr)).To(ContainSubstring("git cat-file -e '"+revision+":"+discoveredPolicyPath+"'"), "stderr:\n%s", stderr)
		Expect(marker).NotTo(BeAnExistingFile(), "git ran the ssh program")
		Expect(packFilesIn(clone)).To(Equal(before), "the run wrote objects into the repository")
	})

	It("fails a scan with the objects it lacks, not with a report that has no policy in it", func() {
		skipUnlessGitCanRefuseLazyFetch()
		clone := discoveredPolicyPartialClone()

		stdout, stderr, exitCode := runCoachBinary(commandPath, clone, os.Environ(), discoveredScanArgs...)

		Expect(exitCode).To(Equal(1), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("Coach does not fetch objects"), "stderr:\n%s", stderr)
		Expect(string(stdout)).To(BeEmpty(), "a report was printed for a read that failed:\n%s", stdout)
	})
})

var _ = Describe("coach codesignal: an older git refuses a partial clone that lacks the discovered policy's objects", func() {
	DescribeTable("without --project-config, names the revision and how many objects it lacks, and fetches nothing",
		func(args []string) {
			env, marker := oldGitFetchingNothing()
			clone := discoveredPolicyPartialClone()
			before := packFilesIn(clone)

			stdout, stderr, exitCode := runCoachBinary(commandPath, clone, env, args...)

			Expect(exitCode).To(Equal(1), "stdout: %s\nstderr: %s", stdout, stderr)
			expectRefusalOnOldGit(stderr, "HEAD has 2 objects that are not in it")
			Expect(marker).NotTo(BeAnExistingFile(), "git ran the ssh program")
			Expect(packFilesIn(clone)).To(Equal(before), "the run wrote objects into the repository")
		},
		Entry("a scan", discoveredScanArgs),
		Entry("--check-project", discoveredCheckProjectArgs),
	)
})
