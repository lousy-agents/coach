package gitrepo

import "testing"

// treelessCloneWhoseTransportLeavesAMarker is a clone made with --filter=tree:0: it has the commits and no tree, so any
// read of a tree is a read of an object the clone lacks. Its remote is one that runs a program of the test's, which
// leaves a marker, so a read that fetches shows.
func treelessCloneWhoseTransportLeavesAMarker(t *testing.T) (clone, marker string) {
	t.Helper()
	source := twoCommitSource(t)
	bare := t.TempDir() + "/bare.git"
	gitOutput(t, source, "clone", "--bare", "--quiet", source, bare)
	gitOutput(t, bare, "config", "uploadpack.allowFilter", "true")
	clone = t.TempDir() + "/treeless"
	gitOutput(t, t.TempDir(), "clone", "--quiet", "--no-checkout", "--filter=tree:0", "file://"+bare, clone)
	gitOutput(t, clone, "remote", "set-url", "origin", "ssh://example.invalid/repo.git")
	return clone, sshMarker(t)
}

// The listing of a tree is a read that git would answer by fetching the tree a partial clone lacks, through whatever
// transport the remote names. Both listings the scan makes are made with the instruction not to, so the transport does not
// run, and the listing fails as a read of an object that is not here.
func TestTheListingsOfATreeNeverRunTheTransportOfAPartialClone(t *testing.T) {
	clone, marker := treelessCloneWhoseTransportLeavesAMarker(t)
	head := gitOutput(t, clone, "rev-parse", "HEAD")

	_, snapshotErr := listSnapshotEntries(clone, "HEAD")
	_, _, trackedErr := DiscoverTrackedFiles(clone, head[:40])

	if snapshotErr == nil || trackedErr == nil {
		t.Errorf("listSnapshotEntries = %v, DiscoverTrackedFiles = %v; want both to fail: the trees of a treeless clone are not here", snapshotErr, trackedErr)
	}
	if transportRan(marker) {
		t.Error("a listing ran the transport of the remote to fetch a tree: the instruction not to fetch was not given")
	}
}
