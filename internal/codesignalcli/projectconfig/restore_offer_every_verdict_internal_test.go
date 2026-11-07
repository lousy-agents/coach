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
	cause    projectreadiness.PolicyCause
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
	cause          projectreadiness.PolicyCause
	saysEverything bool
	printsRestore  bool
}

func summarizeOffer(offer restoreOffer, offered bool, contains []string) offerSummary {
	saysAll := !slices.ContainsFunc(contains, func(part string) bool { return !strings.Contains(offer.text, part) })
	return offerSummary{offered, offer.remedy, offer.cause, saysAll, strings.Contains(offer.text, "git restore")}
}

func (w offerWant) summary() offerSummary {
	return offerSummary{w.offered, w.remedy, w.cause, true, w.command}
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
	if c.verdict == restoreCommittedInvalid {
		judgment.committedInvalid = "document is not valid JSON"
	}
	if c.failureForTheOffer {
		judgment.failure = newGitFailure("what the index holds for it", errors.New("boom"))
	}
	return judgment
}

func verdictCases() []verdictCase {
	cannotRestore := func(cause projectreadiness.PolicyCause, words ...string) offerWant {
		return offerWant{true, projectreadiness.RemedyNone, cause, words, false}
	}
	bothWays := func(cause projectreadiness.PolicyCause, words ...string) (offerWant, offerWant) {
		return cannotRestore(cause, words...), cannotRestore(cause, words...)
	}
	restores := func(words ...string) offerWant {
		return offerWant{true, projectreadiness.RemedyRestore, "", words, true}
	}
	restoresOverEmptyEntry := func(words ...string) offerWant {
		return offerWant{true, projectreadiness.RemedyRestoreOverEmptyEntry, "", words, true}
	}
	staged1, staged2 := bothWays(projectreadiness.PolicyCause("staged_edit"), "staged edit")
	link1, link2 := bothWays(projectreadiness.PolicyCause("committed_as_link"), "symbolic link")
	sparse1, sparse2 := bothWays(projectreadiness.PolicyCause("sparse_checkout"), "sparse checkout")
	merge1, merge2 := bothWays(projectreadiness.PolicyCause("merge_conflict"), "unresolved merge conflict")
	mark1, mark2 := bothWays(projectreadiness.PolicyCause("skip_worktree"), "update-index --no-skip-worktree")
	nonFile1, nonFile2 := bothWays(projectreadiness.PolicyCause("staged_not_a_file"), "not a regular file")
	stagedMark1, stagedMark2 := bothWays(projectreadiness.PolicyCause("skip_worktree"), "marked skip-worktree")
	stagedSparse1, stagedSparse2 := bothWays(projectreadiness.PolicyCause("sparse_checkout"), "staged copy of it that this sparse checkout leaves out")
	emptyMark1, emptyMark2 := bothWays(projectreadiness.PolicyCause("skip_worktree"), "empty entry", "marked skip-worktree", "update-index --no-skip-worktree")
	emptySparse1, emptySparse2 := bothWays(projectreadiness.PolicyCause("sparse_checkout"), "empty entry", "sparse checkout")
	dir1, dir2 := bothWays(projectreadiness.PolicyCause("directory"), "directory", "name a file")
	unmergedNew1, unmergedNew2 := bothWays(projectreadiness.PolicyCause("merge_conflict"), "unresolved merge conflict")
	unknown1, unknown2 := bothWays(projectreadiness.PolicyCause("undetermined"), "could not tell what a restore would do")
	moveFirst := cannotRestore(projectreadiness.PolicyCause("parent_not_directory"), "after moving the file, restore the policy from git")
	return []verdictCase{
		{"nothing committed and nothing staged", restoreNotCommitted, offerWant{}, offerWant{}, false},
		{"a restore that overwrites nothing", restoreOverwritesNothing, restores("restore it with " + restoreSource), moveFirst, false},
		{"a restore over an empty index entry", restoreOverwritesEmptyEntry, restoresOverEmptyEntry("restoring brings back the committed copy", restoreSource), moveFirst, false},
		{"a staged edit a restore would discard", restoreDiscardsStagedEdit, staged1, staged2, false},
		{"a committed link", restoreRecreatesLink, link1, link2, false},
		{"a sparse checkout that leaves the policy out", restoreSkippedBySparseCheckout, sparse1, sparse2, false},
		{"a question git could not answer", restoreUndecidable, cannotRestore(projectreadiness.PolicyCause("git_unreadable"), "could not check"), offerWant{}, true},
		{"a committed unmerged entry", restoreMergeConflict, merge1, merge2, false},
		{"a committed policy marked skip-worktree", restoreSkipWorktreeMarked, mark1, mark2, false},
		{"a staged copy of a policy nothing commits", restoreStagedNotCommitted,
			offerWant{true, projectreadiness.RemedyNone, projectreadiness.PolicyCause("staged_copy"), []string{"staged copy", "restore it with git restore -- "}, true},
			cannotRestore(projectreadiness.PolicyCause("staged_copy"), "move the file that blocks the path"), false},
		{"an unmerged entry for a policy nothing commits", restoreUnmergedNotCommitted, unmergedNew1, unmergedNew2, false},
		{"a staged link of a policy nothing commits", restoreStagedNonFileNotCommitted, nonFile1, nonFile2, false},
		{"a staged copy marked skip-worktree", restoreStagedSkipWorktreeNotCommitted, stagedMark1, stagedMark2, false},
		{"a staged copy a sparse checkout leaves out", restoreStagedSparseNotCommitted, stagedSparse1, stagedSparse2, false},
		{"an empty entry marked skip-worktree for a policy nothing commits", restoreEmptySkipWorktreeNotCommitted, emptyMark1, emptyMark2, false},
		{"an empty entry a sparse checkout leaves out", restoreEmptySparseNotCommitted, emptySparse1, emptySparse2, false},
		{"a directory in the index", restoreDirectoryInIndex, dir1, dir2, false},
		{"a committed copy that is not a valid policy", restoreCommittedInvalid,
			offerWant{true, projectreadiness.RemedyAuthor, "", []string{"committed copy is not a valid policy", "document is not valid JSON", "write a new policy"}, false},
			cannotRestore(projectreadiness.PolicyCause("parent_not_directory"), "committed copy is not a valid policy", "move the file that blocks the path"), false},
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
	cause := [2]MissingCause{MissingAbsent, MissingBlockedParent}[column]

	offer, offered := restoreOfferFor(c.judgment(), "HEAD", &ConfigError{Kind: KindMissing, Missing: cause}, "cfg/p.json")

	if got := summarizeOffer(offer, offered, want.contains); got != want.summary() {
		t.Errorf("offer %q (offered %v, remedy %d, cause %q) does not match %+v: it must say %q and print a git restore command = %v",
			offer.text, offered, offer.remedy, offer.cause, want, want.contains, want.command)
	}
}
