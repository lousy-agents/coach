package projectconfig

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// The sentence that says a committed policy can be brought back is built in one place for the two
// ways it is shown: appended to a missing-policy message as an offer, and in the refusal of compiler
// setup, which names the next action. What it says about the committed copy is the same in both, and
// only the empty-entry variant says why the restore loses nothing.
func TestTheRestoreSentenceIsBuiltOnceForEveryWayItIsShown(t *testing.T) {
	const command = "restore it with git restore --source=abc123 --staged --worktree -- ':(top,literal)pol.json'"
	const emptyEntry = ", and the index holds only an empty entry for it with no content staged, so restoring brings back the committed copy: "
	cases := map[string]struct {
		remedy  projectreadiness.PolicyRemedy
		framing RestoreFraming
		want    string
	}{
		"a restore, as an offer":                      {projectreadiness.RemedyRestore, AsAnOffer, ". It is committed at the analyzed revision: " + command},
		"a restore over an empty entry, as an offer":  {projectreadiness.RemedyRestoreOverEmptyEntry, AsAnOffer, ". It is committed at the analyzed revision" + emptyEntry + command},
		"a restore, as a refusal":                     {projectreadiness.RemedyRestore, AsARefusal("restore_policy"), "; it is committed at the analyzed revision (restore_policy), so " + command},
		"a restore over an empty entry, as a refusal": {projectreadiness.RemedyRestoreOverEmptyEntry, AsARefusal("restore_policy"), "; it is committed at the analyzed revision (restore_policy)" + emptyEntry + command},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireTheSentence(t, c.remedy, c.framing, c.want) })
	}
}

func requireTheSentence(t *testing.T, remedy projectreadiness.PolicyRemedy, framing RestoreFraming, want string) {
	t.Helper()
	if got := RestoreSentence(remedy, "abc123", "pol.json", framing); got != want {
		t.Errorf("RestoreSentence =\n%q\nwant\n%q", got, want)
	}
}
