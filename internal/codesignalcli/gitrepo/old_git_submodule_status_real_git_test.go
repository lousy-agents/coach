package gitrepo

import (
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// aSuperprojectWithATreelessSubmodule commits a submodule whose checkout is a clone made with --filter=tree:0 and no
// checkout, so its root tree is not local, and whose remote is a transport that cannot answer. Anything that has to
// read the submodule's tree has to fetch it.
func aSuperprojectWithATreelessSubmodule(t *testing.T) (superproject, submodule string) {
	t.Helper()
	source := gitfixture.Init(t)
	gitfixture.CommitFile(t, source, "s.txt", "s\n")
	bare := filepath.Join(t.TempDir(), "sub.git")
	gitOutput(t, source, "clone", "--bare", "--quiet", source, bare)
	gitOutput(t, bare, "config", "uploadpack.allowFilter", "true")
	superproject = gitfixture.Init(t)
	gitfixture.CommitFile(t, superproject, "m.txt", "m\n")
	gitOutput(t, superproject, "clone", "--quiet", "--filter=tree:0", "--no-checkout", "file://"+bare, "sub")
	gitOutput(t, superproject, "-c", "protocol.file.allow=always", "submodule", "add", "--quiet", "file://"+bare, "sub")
	gitfixture.Git(t, superproject, "commit", "--quiet", "-m", "add the submodule")
	submodule = filepath.Join(superproject, "sub")
	gitOutput(t, submodule, "remote", "set-url", "origin", "ssh://example.invalid/sub.git")
	return superproject, submodule
}

// `git status` starts a nested status in every populated submodule, and one that is a partial clone lacking its tree
// is fetched into by a git that ignores the instruction not to fetch. So the status for such a git does not go into
// submodules. The git is the real one, run as an old one runs it, and the transport leaves a marker.
func TestTheStatusForAGitBefore244DoesNotFetchIntoATreelessSubmodule(t *testing.T) {
	superproject, submodule := aSuperprojectWithATreelessSubmodule(t)
	asGitBefore244(t)
	marker := sshMarker(t)
	before := objectsIn(t, submodule)

	_, err := worktreeStatusOf(superproject, false)

	if err != nil || transportRan(marker) || objectsIn(t, submodule) != before {
		t.Errorf("worktreeStatusOf = %v; want the status to go on, with no transport run and no object written", err)
	}
}

// The canary is live: the plain status of the same git does go into the submodule and fetch.
func TestThePlainStatusOfAGitThatIgnoresTheInstructionFetchesIntoATreelessSubmodule(t *testing.T) {
	superproject, _ := aSuperprojectWithATreelessSubmodule(t)
	asGitBefore244(t)
	marker := sshMarker(t)

	_, _ = worktreeStatusOf(superproject, true)

	if !transportRan(marker) {
		t.Errorf("the transport did not run: a test that asserts it did not proves nothing")
	}
}
