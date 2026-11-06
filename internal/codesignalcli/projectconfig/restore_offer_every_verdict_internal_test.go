package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// Every verdict has an offer that says what it does, with and without a file in
// the way of the path. The offers are the part of the message a user pastes, so the
// order of the arms that choose among them matters: an arm that prints a restore
// ahead of the one for a file in the way writes into that file's place. This
// walks every verdict both ways instead of trusting the order.
func TestEveryVerdictHasAnOfferThatSaysWhatItDoes(t *testing.T) {
	type offerWant struct {
		offered  bool
		remedy   projectreadiness.PolicyRemedy
		contains []string
		command  bool // a git restore command is printed
	}
	const restoreSource = "git restore --source="
	cases := []struct {
		name               string
		verdict            restoreVerdict
		open, blocked      offerWant
		failureForTheOffer bool
	}{
		{"nothing committed and nothing staged", restoreNotCommitted, offerWant{}, offerWant{}, false},
		{"a restore that overwrites nothing", restoreOverwritesNothing,
			offerWant{true, projectreadiness.RemedyRestore, []string{"restore it with " + restoreSource}, true},
			offerWant{true, projectreadiness.RemedyNone, []string{"after moving the file, restore the policy from git"}, false}, false},
		{"a restore over an empty index entry", restoreOverwritesEmptyEntry,
			offerWant{true, projectreadiness.RemedyRestore, []string{"restoring brings back the committed copy", restoreSource}, true},
			offerWant{true, projectreadiness.RemedyNone, []string{"after moving the file, restore the policy from git"}, false}, false},
		{"a staged edit a restore would discard", restoreDiscardsStagedEdit,
			offerWant{true, projectreadiness.RemedyNone, []string{"staged edit"}, false}, offerWant{true, projectreadiness.RemedyNone, []string{"staged edit"}, false}, false},
		{"a committed link", restoreRecreatesLink,
			offerWant{true, projectreadiness.RemedyNone, []string{"symbolic link"}, false}, offerWant{true, projectreadiness.RemedyNone, []string{"symbolic link"}, false}, false},
		{"a sparse checkout that leaves the policy out", restoreSkippedBySparseCheckout,
			offerWant{true, projectreadiness.RemedyNone, []string{"sparse checkout"}, false}, offerWant{true, projectreadiness.RemedyNone, []string{"sparse checkout"}, false}, false},
		{"a question git could not answer", restoreUndecidable,
			offerWant{true, projectreadiness.RemedyNone, []string{"could not check"}, false}, offerWant{}, true},
		{"a committed unmerged entry", restoreMergeConflict,
			offerWant{true, projectreadiness.RemedyNone, []string{"unresolved merge conflict"}, false}, offerWant{true, projectreadiness.RemedyNone, []string{"unresolved merge conflict"}, false}, false},
		{"a committed policy marked skip-worktree", restoreSkipWorktreeMarked,
			offerWant{true, projectreadiness.RemedyNone, []string{"git update-index --no-skip-worktree"}, false}, offerWant{true, projectreadiness.RemedyNone, []string{"git update-index --no-skip-worktree"}, false}, false},
		{"a staged copy of a policy nothing commits", restoreStagedNotCommitted,
			offerWant{true, projectreadiness.RemedyNone, []string{"staged copy", "restore it with git restore -- "}, true},
			offerWant{true, projectreadiness.RemedyNone, []string{"move the file that blocks the path"}, false}, false},
		{"an unmerged entry for a policy nothing commits", restoreUnmergedNotCommitted,
			offerWant{true, projectreadiness.RemedyNone, []string{"unresolved merge conflict"}, false}, offerWant{true, projectreadiness.RemedyNone, []string{"unresolved merge conflict"}, false}, false},
		{"a staged link of a policy nothing commits", restoreStagedNonFileNotCommitted,
			offerWant{true, projectreadiness.RemedyNone, []string{"not a regular file"}, false}, offerWant{true, projectreadiness.RemedyNone, []string{"not a regular file"}, false}, false},
		{"a staged copy marked skip-worktree", restoreStagedSkipWorktreeNotCommitted,
			offerWant{true, projectreadiness.RemedyNone, []string{"marked skip-worktree"}, false}, offerWant{true, projectreadiness.RemedyNone, []string{"marked skip-worktree"}, false}, false},
		{"a staged copy a sparse checkout leaves out", restoreStagedSparseNotCommitted,
			offerWant{true, projectreadiness.RemedyNone, []string{"sparse checkout"}, false}, offerWant{true, projectreadiness.RemedyNone, []string{"sparse checkout"}, false}, false},
		{"a verdict the offers do not know", restoreVerdict(99),
			offerWant{true, projectreadiness.RemedyNone, []string{"could not tell what a restore would do"}, false}, offerWant{true, projectreadiness.RemedyNone, []string{"could not tell what a restore would do"}, false}, false},
	}
	known := map[restoreVerdict]bool{}
	for _, c := range cases {
		if c.verdict != restoreVerdict(99) {
			known[c.verdict] = true
		}
	}
	if len(known) != int(restoreVerdictCount) {
		t.Fatalf("the table covers %d of the %d verdicts: a verdict was added without a row here", len(known), restoreVerdictCount)
	}
	for _, c := range cases {
		for _, blocked := range []bool{false, true} {
			want := c.open
			cause := CauseAbsent
			label := c.name + ", nothing in the way"
			if blocked {
				want, cause, label = c.blocked, CauseBlockedParent, c.name+", a file in the way"
			}
			t.Run(label, func(t *testing.T) {
				judgment := policyJudgment{verdict: c.verdict}
				if c.failureForTheOffer {
					judgment.failure = newGitFailure("what the index holds for it", errors.New("boom"))
				}

				offer, offered := restoreOfferFor(judgment, "HEAD", &ConfigError{Kind: KindMissing, Cause: cause}, "cfg/p.json")

				if offered != want.offered {
					t.Fatalf("offered = %v, want %v (offer %+v)", offered, want.offered, offer)
				}
				if !offered {
					return
				}
				if offer.remedy != want.remedy {
					t.Errorf("remedy = %d, want %d", offer.remedy, want.remedy)
				}
				for _, text := range want.contains {
					if !strings.Contains(offer.text, text) {
						t.Errorf("offer %q; want it to contain %q", offer.text, text)
					}
				}
				if prints := strings.Contains(offer.text, "git restore"); prints != want.command {
					t.Errorf("offer %q; prints a git restore command = %v, want %v", offer.text, prints, want.command)
				}
			})
		}
	}
}
