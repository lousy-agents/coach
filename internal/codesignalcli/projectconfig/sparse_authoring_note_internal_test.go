package projectconfig

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// sparseRepository commits cfg/p.json and docs/d.json and, when asked, turns on a sparse checkout
// that leaves cfg out, which removes the directory from the working tree.
func sparseRepository(t *testing.T, sparse bool) string {
	t.Helper()
	repo := committedRepository(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", validPolicy)
	gitfixture.CommitFile(t, repo, "docs/d.json", "{}")
	if sparse {
		gitfixture.Git(t, repo, "sparse-checkout", "set", "--no-cone", "/*", "!/cfg/")
	}
	return repo
}

// A new policy written into a directory the sparse checkout leaves out cannot be staged with a plain
// git add, which refuses a path outside the checkout. The offer to write one is kept, because a new
// policy is the way forward, and it says how to stage it.
func TestANewPolicyInADirectoryOutsideTheSparseCheckoutIsOfferedWithTheWayToStageIt(t *testing.T) {
	cases := map[string]struct {
		sparse   bool
		path     string
		wantNote bool
	}{
		"a directory the sparse checkout leaves out": {true, "cfg/zz.json", true},
		"no sparse checkout":                         {false, "cfg/zz.json", false},
		"a sparse checkout and a policy at the root": {true, "zz.json", false},
		"a sparse checkout and a directory on disk":  {true, "docs/zz.json", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireTheNote(t, sparseRepository(t, c.sparse), c.path, c.wantNote) })
	}
}

func requireTheNote(t *testing.T, repo, path string, wantNote bool) {
	t.Helper()

	status, err := ExplicitOnDiskStatus(NewGitRunner(), repo, "HEAD", path)

	if err != nil || status == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want a missing policy", status, err)
	}
	hasNote := strings.Contains(status.Message, "git add --sparse")
	if hasNote != wantNote || status.Remedy != projectreadiness.RemedyAuthor || status.PolicyCause != "" {
		t.Errorf("message %q, remedy %d, cause %q; want a new policy offered, and the note about staging it = %v", status.Message, status.Remedy, status.PolicyCause, wantNote)
	}
}

// The note says what to do if the directory is outside the checkout, and not that it is: a new
// directory that was never in the repository looks the same from here.
func TestTheSparseNoteIsConditionalAndNamesBothWaysOut(t *testing.T) {
	status, err := ExplicitOnDiskStatus(NewGitRunner(), sparseRepository(t, true), "HEAD", "cfg/zz.json")

	if err != nil || status == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v", status, err)
	}
	for _, part := range []string{"If its directory is outside the sparse checkout", "git sparse-checkout add", "git add --sparse"} {
		if !strings.Contains(status.Message, part) {
			t.Errorf("message %q does not say %q", status.Message, part)
		}
	}
}

// The note belongs to the offer of a new policy for a path nothing commits, and to no other verdict:
// a policy that is committed, staged or in conflict keeps what its own offer says, and a file where
// the directory should be keeps its refusal.
func TestTheSparseNoteIsOnlyForAPolicyNothingCommits(t *testing.T) {
	missing := &ConfigError{Kind: KindMissing, Missing: MissingAbsent, Remedy: projectreadiness.RemedyAuthor}
	blocked := &ConfigError{Kind: KindMissing, Missing: MissingBlockedParent, Remedy: projectreadiness.RemedyNone}

	if offer, offered := restoreOfferFor(policyJudgment{verdict: restoreDiscardsStagedEdit, sparseNewPolicy: true}, "HEAD", missing, "cfg/p.json"); !offered || offer != stagedEditOffer("HEAD", "cfg/p.json", false) {
		t.Errorf("a staged edit with the flag set: offer %+v, offered %v; want the staged-edit offer", offer, offered)
	}
	if offer, offered := restoreOfferFor(policyJudgment{verdict: restoreNotCommitted, sparseNewPolicy: true}, "HEAD", blocked, "cfg/p.json"); offered {
		t.Errorf("a file in the way with the flag set: offer %+v; want none, since no new policy fits there", offer)
	}
}

// Whether a new policy may need git add --sparse is asked of a path nothing commits only: a committed
// policy that the sparse checkout leaves out has its own verdict and is not told to author one.
func TestAPolicyTheSparseCheckoutLeavesOutIsNotJudgedAsANewPolicy(t *testing.T) {
	judgment := judgeRestore(NewGitRunner(), sparseRepository(t, true), "HEAD", "cfg/p.json")

	if judgment.sparseNewPolicy || judgment.verdict == restoreNotCommitted {
		t.Errorf("judgment = %+v; want the verdict of a committed policy and no sparse note for a new one", judgment)
	}
}
