package projectconfig

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A parent spelled in another case is reported only for a policy that is missing. When the entry is there and
// is something else (here a link, under a parent the disk folds to another case), what it is comes first: the
// user has two things to fix, and the spelling of a parent is not the one that decides whether the entry
// can be a policy.
func TestAParentSpelledInAnotherCaseDoesNotHideWhatTheEntryIs(t *testing.T) {
	disk := foldingDisk{fstest.MapFS{"CFG/p.json": &fstest.MapFile{Mode: fs.ModeSymlink, Data: []byte("target.json")}}}

	found := configErrorOf(t, checkRegularFileThrough(disk, "cfg/p.json"))

	if found.PolicyCause != projectreadiness.CauseSymbolicLink {
		t.Errorf("cause %q in %q; want the link named before the parent's spelling", found.PolicyCause, found.Message)
	}
}
