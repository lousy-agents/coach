package main

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// refusedInAPartialCloneWith runs a diff in a partial clone whose merge base lacks objects, in an environment
// with the extra settings, and requires the refusal the clone gets without them: exit 1, the objects named,
// nothing printed as a report, and nothing fetched.
func refusedInAPartialCloneWith(extraEnvironment ...string) {
	skipUnlessGitCanRefuseLazyFetch()
	clone := changedFilePartialClone()
	before := packFilesIn(clone)

	stdout, stderr, exitCode := runCoachBinary(commandPath, clone, append(os.Environ(), extraEnvironment...), diffArgs...)

	Expect(exitCode).To(Equal(1), "stdout: %s\nstderr: %s", stdout, stderr)
	Expect(string(stderr)).To(ContainSubstring("the merge base of HEAD~1 and HEAD has 1 object that is not in it"), "stderr:\n%s", stderr)
	Expect(string(stdout)).To(BeEmpty(), "a report was printed for a run that analyzed none of the change:\n%s", stdout)
	Expect(packFilesIn(clone)).To(Equal(before), "the run wrote objects into the repository")
}

// GIT_CONFIG makes `git config` read the one file it names, and no other git command honours it. The check for
// a partial clone reads the repository's configuration with git config, so a caller that exports it had the
// check answer "not a partial clone" while every other read still lacked the objects: the diff recorded its
// changed files as unreadable and exited 0. The other ways a caller supplies configuration reach every git
// command, the check included, so they cannot make it disagree with the reads it protects.
var _ = Describe("coach codesignal: configuration in the caller's environment does not hide a partial clone", func() {
	It("is not hidden by a GIT_CONFIG file", func() {
		unrelated := filepath.Join(GinkgoT().TempDir(), "unrelated.cfg")
		Expect(os.WriteFile(unrelated, []byte("[user]\n\tname = someone\n"), 0o644)).To(Succeed())

		refusedInAPartialCloneWith("GIT_CONFIG=" + unrelated)
	})

	It("is not hidden by GIT_CONFIG_COUNT saying the remote is not a promisor", func() {
		refusedInAPartialCloneWith("GIT_CONFIG_COUNT=1", "GIT_CONFIG_KEY_0=remote.origin.promisor", "GIT_CONFIG_VALUE_0=false")
	})

	It("is not hidden by GIT_CONFIG_PARAMETERS saying the remote is not a promisor", func() {
		refusedInAPartialCloneWith("GIT_CONFIG_PARAMETERS='remote.origin.promisor=false'")
	})
})
