package gitrepo

import (
	"os"
	"strings"
	"testing"
)

func transportRan(marker string) bool {
	_, err := os.Stat(marker)
	return err == nil
}

// A git before 2.44 does not read the instruction not to fetch, and finding the merge base reads commits, so a base
// commit that only the remote has is fetched by the question that asks for the merge base. This runs the real git
// as that git runs it, with a transport that leaves a marker, and asks what the guard does about such a base.
func TestOnAnOldGitABaseCommitOnlyTheRemoteHasIsRefusedWithoutRunningTheTransport(t *testing.T) {
	clone, bare := filteredCloneAndItsBare(t, twoCommitSource(t), true)
	base := commitOnlyTheRemoteHas(t, bare)
	asGitBefore244(t)
	marker := sshMarker(t)
	before := objectsIn(t, clone)

	err := refuseWhenReadRevisionsLackObjects(gitBefore244, clone, base, olderThanTheInstruction)

	if !strings.Contains(refusalText(err), "the commit --base names has 1 object that is not in it") || !strings.Contains(refusalText(err), base) {
		t.Errorf("refuseWhenReadRevisionsLackObjects = %v; want the base commit refused, and named", err)
	}
	if transportRan(marker) {
		t.Errorf("git ran the transport to fetch the base commit")
	}
	if after := objectsIn(t, clone); after != before {
		t.Errorf("the guard wrote objects: %d before, %d after", before, after)
	}
}

// The canary is live: asked for the merge base first, as the guard once did, the same git fetches the commit.
func TestTheMergeBaseQuestionFetchesTheBaseCommitOnAGitThatIgnoresTheInstruction(t *testing.T) {
	clone, bare := filteredCloneAndItsBare(t, twoCommitSource(t), true)
	base := commitOnlyTheRemoteHas(t, bare)
	asGitBefore244(t)
	marker := sshMarker(t)

	_, _ = gitBefore244(clone, "merge-base", "--", base, "HEAD")

	if !transportRan(marker) {
		t.Errorf("the transport did not run: a test that asserts it did not proves nothing")
	}
}

func TestOnAnOldGitABaseCommitThatIsHereGoesOnToTheMergeBaseWithoutFetching(t *testing.T) {
	clone, _ := filteredCloneAndItsBare(t, twoCommitSource(t), true)
	asGitBefore244(t)
	marker := sshMarker(t)
	before := objectsIn(t, clone)

	err := refuseWhenReadRevisionsLackObjects(gitBefore244, clone, "HEAD", olderThanTheInstruction)

	if err != nil || transportRan(marker) || objectsIn(t, clone) != before {
		t.Errorf("refuseWhenReadRevisionsLackObjects = %v; want a base that is here read, with nothing run and nothing written", err)
	}
}
