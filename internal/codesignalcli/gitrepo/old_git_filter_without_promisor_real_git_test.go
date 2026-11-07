package gitrepo

import (
	"strings"
	"testing"
)

// Git takes a remote with remote.<name>.partialclonefilter for a promisor remote whatever remote.<name>.promisor
// says, and a git before 2.44 fetches a missing object from it. The configuration Coach reads says the clone is
// ordinary; the probe decides, and nothing is fetched.
func TestOnAnOldGitACloneWhoseRemoteHasAFilterAndPromisorFalseIsRefusedWithoutFetching(t *testing.T) {
	clone := filteredClone(t, twoCommitSource(t), false)
	gitOutput(t, clone, "config", "remote.origin.promisor", "false")
	if !strings.Contains(gitOutput(t, clone, "config", "--get", "remote.origin.partialclonefilter"), "blob:none") {
		t.Fatal("the clone must keep its filter, so that git takes the remote for a promisor")
	}
	asGitBefore244(t)
	marker := sshMarker(t)
	before := objectsIn(t, clone)

	err := refuseWhenReadRevisionsLackObjects(gitBefore244, clone, "", olderThanTheInstruction)

	if !strings.Contains(refusalText(err), "git 2.43.0 cannot be told not to fetch missing objects") || !strings.Contains(refusalText(err), "use git 2.44 or later") {
		t.Errorf("refuseWhenReadRevisionsLackObjects = %v; want the missing objects refused, whatever the configuration says", err)
	}
	if transportRan(marker) || objectsIn(t, clone) != before {
		t.Errorf("the guard ran the transport or wrote objects")
	}
}

func TestOnAnOldGitACloneThatLacksNothingIsReadWhateverItsConfiguration(t *testing.T) {
	clone := filteredClone(t, twoCommitSource(t), true)
	gitOutput(t, clone, "config", "remote.origin.promisor", "false")
	asGitBefore244(t)
	marker := sshMarker(t)
	before := objectsIn(t, clone)

	err := refuseWhenReadRevisionsLackObjects(gitBefore244, clone, "", olderThanTheInstruction)

	if err != nil || transportRan(marker) || objectsIn(t, clone) != before {
		t.Errorf("refuseWhenReadRevisionsLackObjects = %v; want the clone read, with nothing run and nothing written", err)
	}
}
