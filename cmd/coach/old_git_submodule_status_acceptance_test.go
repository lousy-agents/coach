package main

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// superprojectWithATreelessSubmodule is a repository that commits a submodule whose checkout is a clone made with
// --filter=tree:0 and no checkout, so its root tree is not local, and whose remote is a program that leaves a
// marker when git runs it. Anything that has to read the submodule's tree fetches it.
func superprojectWithATreelessSubmodule(remote string) string {
	GinkgoHelper()
	source := newTempGitRepo()
	commitFile(source, "s.go", minimalGoSource)
	bare := filepath.Join(GinkgoT().TempDir(), "sub.git")
	gitOutputIn(source, "clone", "--bare", "--quiet", source, bare)
	gitOutputIn(bare, "config", "uploadpack.allowFilter", "true")

	super := commitScannableGoRepo()
	gitOutputIn(super, "clone", "--quiet", "--filter=tree:0", "--no-checkout", "file://"+bare, "sub")
	gitOutputIn(super, "-c", "protocol.file.allow=always", "submodule", "add", "--quiet", "file://"+bare, "sub")
	gitOutputIn(super, "commit", "--quiet", "-m", "add the submodule")
	gitOutputIn(filepath.Join(super, "sub"), "remote", "set-url", "origin", remote)
	return super
}

// A scan runs `git status` in the superproject to say what is uncommitted, and `git status` goes into every
// populated submodule. A git before 2.44 ignores GIT_NO_LAZY_FETCH, and a submodule that is a partial clone
// lacking objects is fetched into: the remote's transport runs and objects are written, though the guard has
// said that nothing is fetched on that git, having only looked at the superproject's revisions. So on such a git
// the status does not go into submodules.
var _ = Describe("coach codesignal: on a git before 2.44 the status of the worktree does not fetch into a partial-clone submodule", func() {
	It("runs no transport and writes no object, and the scan goes on", func() {
		sshCommand, marker := sshProgramThatLeavesAMarker()
		super := superprojectWithATreelessSubmodule("ssh://example.invalid/sub.git")
		submodule := filepath.Join(super, "sub")
		before := objectCount(submodule)

		stdout, stderr, exitCode := runCoachBinary(commandPath, super, append(gitBefore244(), "GIT_SSH_COMMAND="+sshCommand), "codesignal", "--baseline", "--format=json", "--no-interactive")

		Expect(exitCode).To(Equal(0), "stdout: %s\nstderr: %s", stdout, stderr)
		Expect(marker).NotTo(BeAnExistingFile(), "git ran the ssh program to fetch into the submodule")
		Expect(objectCount(submodule)).To(Equal(before), "the scan wrote objects into the submodule")
	})
})
