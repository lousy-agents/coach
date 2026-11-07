package gitrepo

import "testing"

// The way out of a refusal for a git that cannot be told not to fetch is a newer git, and the fetch when there is
// one to paste: a sentence that ended with ", or " for want of a fetch would be broken.
func TestTheWayOutIsTheNewerGitAndTheFetchWhenThereIsOne(t *testing.T) {
	if got := wayOut(""); got != "use git 2.44 or later" {
		t.Errorf("wayOut(\"\") = %q; want the newer git alone, with nothing after it", got)
	}
	if got := wayOut("fetch the revision's files"); got != "use git 2.44 or later, or fetch the revision's files" {
		t.Errorf("wayOut(fetch) = %q; want the newer git, then the fetch after a comma", got)
	}
}
