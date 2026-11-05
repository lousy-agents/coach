package gitrepo

import "testing"

// `git status` starts a nested status in every populated submodule, and one that is a partial clone lacking its tree
// is fetched into by a git that ignores the instruction not to fetch. So the status a --worktree scan reads from such
// a git does not go into submodules. The git is the real one, run as an old one runs it, and the transport leaves a
// marker.
func TestTheWorktreeStatusForAGitBefore244DoesNotFetchIntoATreelessSubmodule(t *testing.T) {
	superproject, submodule := aSuperprojectWithATreelessSubmodule(t)
	asGitBefore244(t)
	marker := sshMarker(t)
	before := objectsIn(t, submodule)

	_, err := readWorktreeStatusBy(worktreeChildOf(commandTold(nil)), superproject, false, olderThanTheInstruction)

	if err != nil || transportRan(marker) || objectsIn(t, submodule) != before {
		t.Errorf("readWorktreeStatusBy = %v; want the status to go on, with no transport run and no object written", err)
	}
}

// The canary is live: the arguments of a git that can be told not to fetch, given to a git that ignores the
// instruction, go into the submodule and fetch.
func TestTheWorktreeStatusArgumentsOfAGitThatCanBeToldNotToFetchFetchIntoATreelessSubmoduleWhenGitIgnoresTheInstruction(t *testing.T) {
	superproject, _ := aSuperprojectWithATreelessSubmodule(t)
	asGitBefore244(t)
	marker := sshMarker(t)

	_, _ = readWorktreeStatusBy(worktreeChildOf(commandTold(nil)), superproject, true, "2.54.0")

	if !transportRan(marker) {
		t.Errorf("the transport did not run: a test that asserts it did not proves nothing")
	}
}

func TestTheWorktreeStatusForAGitThatCanBeToldNotToFetchLooksIntoATreelessSubmoduleWithoutFetching(t *testing.T) {
	superproject, submodule := aSuperprojectWithATreelessSubmodule(t)
	marker := sshMarker(t)
	before := objectsIn(t, submodule)

	_, _ = ReadWorktreeStatus(superproject)

	if transportRan(marker) || objectsIn(t, submodule) != before {
		t.Errorf("the status ran the transport or wrote an object, though git was told not to fetch")
	}
}
