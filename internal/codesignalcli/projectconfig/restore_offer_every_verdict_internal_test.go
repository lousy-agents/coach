package projectconfig

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

const restoreSource = "git restore --source="

// A verdict no offer knows, for the row that holds the default.
const unknownVerdict = restoreVerdict(99)

// offerWant is what the offer for a verdict says.
type offerWant struct {
	offered  bool
	remedy   projectreadiness.PolicyRemedy
	contains []string
	command  bool // a git restore command is printed
}

// verdictCase is one verdict and what its offer says with nothing in the way of the
// path and with a file where its directory should be.
type verdictCase struct {
	name               string
	verdict            restoreVerdict
	open, blocked      offerWant
	failureForTheOffer bool
}

// offerSummary is an offer reduced to what is compared: whether there is one, the
// remedy, whether it says everything the row requires, and whether it prints a restore.
type offerSummary struct {
	offered        bool
	remedy         projectreadiness.PolicyRemedy
	saysEverything bool
	printsRestore  bool
}

func summarizeOffer(offer restoreOffer, offered bool, contains []string) offerSummary {
	saysAll := !slices.ContainsFunc(contains, func(part string) bool { return !strings.Contains(offer.text, part) })
	return offerSummary{offered, offer.remedy, saysAll, strings.Contains(offer.text, "git restore")}
}

func (w offerWant) summary() offerSummary {
	return offerSummary{w.offered, w.remedy, true, w.command}
}

// blockedIndex picks the column of the pairs below.
func blockedIndex(blocked bool) int {
	if blocked {
		return 1
	}
	return 0
}

func (c verdictCase) judgment() policyJudgment {
	judgment := policyJudgment{verdict: c.verdict}
	if c.failureForTheOffer {
		judgment.failure = newGitFailure("what the index holds for it", errors.New("boom"))
	}
	return judgment
}

func verdictCases() []verdictCase {
	none := projectreadiness.RemedyNone
	cannotRestore := func(words ...string) offerWant { return offerWant{true, none, words, false} }
	bothWays := func(words ...string) (offerWant, offerWant) { return cannotRestore(words...), cannotRestore(words...) }
	staged1, staged2 := bothWays("staged edit")
	link1, link2 := bothWays("symbolic link")
	sparse1, sparse2 := bothWays("sparse checkout")
	merge1, merge2 := bothWays("unresolved merge conflict")
	mark1, mark2 := bothWays("git update-index --no-skip-worktree")
	nonFile1, nonFile2 := bothWays("not a regular file")
	stagedMark1, stagedMark2 := bothWays("marked skip-worktree")
	dir1, dir2 := bothWays("directory", "name a file")
	emptyMark1, emptyMark2 := bothWays("empty entry", "marked skip-worktree", "git update-index --no-skip-worktree")
	emptySparse1, emptySparse2 := bothWays("empty entry", "sparse checkout")
	unknown1, unknown2 := bothWays("could not tell what a restore would do")
	moveFirst := cannotRestore("after moving the file, restore the policy from git")
	return []verdictCase{
		{"nothing committed and nothing staged", restoreNotCommitted, offerWant{}, offerWant{}, false},
		{"a restore that overwrites nothing", restoreOverwritesNothing,
			offerWant{true, projectreadiness.RemedyRestore, []string{"restore it with " + restoreSource}, true}, moveFirst, false},
		{"a restore over an empty index entry", restoreOverwritesEmptyEntry,
			offerWant{true, projectreadiness.RemedyRestore, []string{"restoring brings back the committed copy", restoreSource}, true}, moveFirst, false},
		{"a staged edit a restore would discard", restoreDiscardsStagedEdit, staged1, staged2, false},
		{"a committed link", restoreRecreatesLink, link1, link2, false},
		{"a sparse checkout that leaves the policy out", restoreSkippedBySparseCheckout, sparse1, sparse2, false},
		{"a question git could not answer", restoreUndecidable, cannotRestore("could not check"), offerWant{}, true},
		{"a committed unmerged entry", restoreMergeConflict, merge1, merge2, false},
		{"a committed policy marked skip-worktree", restoreSkipWorktreeMarked, mark1, mark2, false},
		{"a staged copy of a policy nothing commits", restoreStagedNotCommitted,
			offerWant{true, none, []string{"staged copy", "restore it with git restore -- "}, true}, cannotRestore("move the file that blocks the path"), false},
		{"an unmerged entry for a policy nothing commits", restoreUnmergedNotCommitted, merge1, merge2, false},
		{"a staged link of a policy nothing commits", restoreStagedNonFileNotCommitted, nonFile1, nonFile2, false},
		{"a staged copy marked skip-worktree", restoreStagedSkipWorktreeNotCommitted, stagedMark1, stagedMark2, false},
		{"a staged copy a sparse checkout leaves out", restoreStagedSparseNotCommitted, sparse1, sparse2, false},
		{"an empty entry marked skip-worktree for a policy nothing commits", restoreEmptySkipWorktreeNotCommitted, emptyMark1, emptyMark2, false},
		{"an empty entry a sparse checkout leaves out", restoreEmptySparseNotCommitted, emptySparse1, emptySparse2, false},
		{"a directory in the index", restoreDirectoryInIndex, dir1, dir2, false},
		{"a verdict the offers do not know", unknownVerdict, unknown1, unknown2, false},
	}
}

// Every verdict has an offer that says what it does, with and without a file in
// the way of the path. The offers are the part of the message a user pastes, so the
// order of the arms that choose among them matters: an arm that prints a restore
// ahead of the one for a file in the way writes into that file's place. This
// walks every verdict both ways instead of trusting the order.
func TestEveryVerdictHasAnOfferThatSaysWhatItDoes(t *testing.T) {
	cases := verdictCases()
	requireARowForEveryVerdict(t, cases)
	for _, c := range cases {
		runWithAndWithoutAFileInTheWay(t, c)
	}
}

func requireARowForEveryVerdict(t *testing.T, cases []verdictCase) {
	t.Helper()
	known := map[restoreVerdict]bool{}
	for _, c := range cases {
		known[c.verdict] = true
	}
	delete(known, unknownVerdict)
	if len(known) != int(restoreVerdictCount) {
		t.Fatalf("the table covers %d of the %d verdicts: a verdict was added without a row here", len(known), restoreVerdictCount)
	}
}

func runWithAndWithoutAFileInTheWay(t *testing.T, c verdictCase) {
	t.Helper()
	for _, blocked := range []bool{false, true} {
		label := [2]string{c.name + ", nothing in the way", c.name + ", a file in the way"}[blockedIndex(blocked)]
		t.Run(label, func(t *testing.T) { assertOfferFor(t, c, blocked) })
	}
}

func assertOfferFor(t *testing.T, c verdictCase, blocked bool) {
	t.Helper()
	column := blockedIndex(blocked)
	want := [2]offerWant{c.open, c.blocked}[column]
	cause := [2]MissingCause{CauseAbsent, CauseBlockedParent}[column]

	offer, offered := restoreOfferFor(c.judgment(), "HEAD", &ConfigError{Kind: KindMissing, Cause: cause}, "cfg/p.json")

	if got := summarizeOffer(offer, offered, want.contains); got != want.summary() {
		t.Errorf("offer %q (offered %v, remedy %d) does not match %+v: it must say %q and print a git restore command = %v",
			offer.text, offered, offer.remedy, want, want.contains, want.command)
	}
}
