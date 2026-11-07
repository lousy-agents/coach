package main

import (
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// commitThatOnlyTheRemoteHas adds a commit to the bare repository that clone was made from, which the clone has
// not fetched, and returns its id: the shape of a --base that names a pull request's head or a branch the
// checkout did not bring.
func commitThatOnlyTheRemoteHas(bare string) string {
	GinkgoHelper()
	tree := strings.TrimSpace(gitOutputIn(bare, "rev-parse", "HEAD^{tree}"))
	parent := strings.TrimSpace(gitOutputIn(bare, "rev-parse", "HEAD"))
	return strings.TrimSpace(gitOutputIn(bare, "commit-tree", "-p", parent, "-m", "only the remote has this", tree))
}

// A git before 2.44 ignores GIT_NO_LAZY_FETCH and fetches any commit or object it parses and does not have. The
// merge base is found by walking commits from the base and from HEAD, so asking for it before asking whether the
// base is here ran the remote's transport and wrote objects, and only then refused. The base is asked about first,
// with questions that fetch nothing.
var _ = Describe("coach codesignal: on a git before 2.44 a --base commit that is not here is refused before anything is fetched", func() {
	It("runs no transport and writes no object", func() {
		sshCommand, marker := sshProgramThatLeavesAMarker()
		var bare string
		clone := filteredCloneOf(commitScannableGoRepo(), func(path string) string {
			bare = path
			return "ssh://example.invalid/repo.git"
		})
		base := commitThatOnlyTheRemoteHas(bare)
		before := objectCount(clone)

		stdout, stderr, exitCode := runCoachBinary(commandPath, clone, append(gitBefore244(), "GIT_SSH_COMMAND="+sshCommand), "codesignal", "--base", base, "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(1), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(string(stderr)).To(ContainSubstring("git 2.40.1 cannot be told not to fetch missing objects"), "stderr:\n%s", stderr)
		Expect(marker).NotTo(BeAnExistingFile(), "git ran the ssh program to fetch the base")
		Expect(objectCount(clone)).To(Equal(before), "the run wrote objects into the repository")
	})
})
