package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// The index is asked two questions about a policy missing from disk, whether the path lies inside a submodule
// and what the index holds for it. Real git cannot fail the second alone, so it is specified here with a fake
// that is the whole of git for the check: when only the listing of the entry fails, the message says what could
// not be checked, and offers neither a restore nor a new policy, whatever the revision holds.
func TestAnIndexEntryThatCannotBeListedOffersNeitherARestoreNorANewPolicy(t *testing.T) {
	cases := map[string]indexFake{
		"a policy committed and deleted from disk":       {entryListingErr: errors.New("boom: entry unreadable")},
		"a policy nothing commits, with nothing on disk": {entryListingErr: errors.New("boom: entry unreadable"), uncommitted: true},
	}
	for name, fake := range cases {
		t.Run(name, func(t *testing.T) { requireNeitherOfferWhenTheEntryCannotBeListed(t, fake) })
	}
}

func requireNeitherOfferWhenTheEntryCannotBeListed(t *testing.T, fake indexFake) {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")

	status, err := ExplicitOnDiskStatus(fake.run, repo, "HEAD", "cfg/p.json")

	if err != nil || status == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want a missing-policy error that carries the git failure", status, err)
	}
	if !strings.Contains(status.Message, "could not check what the index holds") || !strings.Contains(status.Message, "boom: entry unreadable") {
		t.Errorf("message %q; want it to say what could not be checked and carry git's error", status.Message)
	}
	if strings.Contains(status.Message, "git restore") || status.OffersAuthoring() {
		t.Errorf("message %q, offers authoring %v; want neither a restore nor a new policy over an index git could not read", status.Message, status.OffersAuthoring())
	}
}
