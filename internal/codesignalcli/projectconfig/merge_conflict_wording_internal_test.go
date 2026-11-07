package projectconfig

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

var (
	bothSides   = conflictSides{ours: true, theirs: true}
	onlyOurs    = conflictSides{ours: true}
	onlyTheirs  = conflictSides{theirs: true}
	neitherSide = conflictSides{}
)

const conflictedFile = `"cfg/p.json"`

// unprintedFile is how the advice refers to a file whose name has a character a terminal would act on.
const unprintedFile = "the conflicted file (the name has a character that is not printed here)"

func acceptingTheDeletionOf(file string) string {
	return "accept the deletion by recording " + file + " as deleted and committing"
}

// A policy in a merge conflict is missing from disk (that is the state the message describes), so there is no file
// to edit, and marking the path resolved while it is not there records a deletion and drops the other side. The
// message says to bring the file back with its conflict markers first, and describes each step in words.
func TestTheMergeConflictOffersBringTheConflictedFileBackBeforeAskingForItToBeResolved(t *testing.T) {
	const advice = "bring back " + conflictedFile + " with the conflict markers git makes from the two sides, resolve the conflict in it, mark it resolved, and commit"
	offers := map[string]restoreOffer{
		"over a committed policy":      mergeConflictOffer("cfg/p.json", bothSides, false),
		"for a policy nothing commits": unmergedNotCommittedOffer("cfg/p.json", bothSides, false),
	}
	for name, offer := range offers {
		t.Run(name, func(t *testing.T) { requireTheAdviceThenRunAgain(t, offer, advice) })
	}
}

// requireTheAdviceThenRunAgain holds the whole text after the first sentence: the advice, then the instruction to run
// again, and nothing else in between, so no command rides along with the words.
func requireTheAdviceThenRunAgain(t *testing.T, offer restoreOffer, advice string) {
	t.Helper()
	if !strings.HasSuffix(offer.text, "; "+advice+", then run again") {
		t.Errorf("offer %q; want it to end with %q and the instruction to run again", offer.text, advice)
	}
}

func TestAConflictedNameThatCannotBePrintedGetsTheStepWithoutTheName(t *testing.T) {
	offer := mergeConflictOffer("cfg/p\x1b.json", bothSides, false)

	requireTheAdviceThenRunAgain(t, offer, "bring back "+unprintedFile+" with the conflict markers git makes from the two sides, resolve the conflict in it, mark it resolved, and commit")
	if strings.Contains(offer.text, "\x1b") || strings.Contains(offer.text, "p.json") {
		t.Errorf("offer %q; want the unprintable name left out", offer.text)
	}
}

// A path the policy check refuses names nothing the scan could use, so it is left out as well, though every character in
// it can be shown.
func TestAConflictedNameThatThePolicyCheckRefusesGetsTheStepWithoutTheName(t *testing.T) {
	offer := mergeConflictOffer("../outside.json", bothSides, false)

	requireTheAdviceThenRunAgain(t, offer, "bring back "+unprintedFile+" with the conflict markers git makes from the two sides, resolve the conflict in it, mark it resolved, and commit")
	if strings.Contains(offer.text, "outside.json") {
		t.Errorf("offer %q; want the refused name left out", offer.text)
	}
}

// oneSideCase is an offer for a conflict that one side deleted the file in, and the advice it is to give.
type oneSideCase struct {
	name  string
	offer restoreOffer
	want  string
}

func oneSideCases() []oneSideCase {
	keepOf := func(side string) string {
		return "only one side of the conflict still has the file, so there are no conflict markers; keep the version of " + conflictedFile +
			" that " + side + " holds by bringing it back, mark it resolved, and commit, or " + acceptingTheDeletionOf(conflictedFile)
	}
	sidesAndAdvice := map[string]struct {
		sides conflictSides
		want  string
	}{
		"only the merged side holds it":  {onlyTheirs, keepOf("the side being merged")},
		"only the current side holds it": {onlyOurs, keepOf("the side being merged into")},
		"neither side holds it":          {neitherSide, "both sides deleted it; " + acceptingTheDeletionOf(conflictedFile)},
	}
	var cases []oneSideCase
	for name, c := range sidesAndAdvice {
		cases = append(cases,
			oneSideCase{name + " over a committed policy", mergeConflictOffer("cfg/p.json", c.sides, false), c.want},
			oneSideCase{name + " for a policy nothing commits", unmergedNotCommittedOffer("cfg/p.json", c.sides, false), c.want})
	}
	return cases
}

// Conflict markers are made from the versions of both sides, and there are none to make when one has no version, and
// then there is nothing to resolve. The advice for a file one side deleted keeps the version that is left or accepts
// the deletion, and names the side that still has it.
func TestAFileOneSideDeletedGetsTheStepsThatWorkAndNoConflictMarkers(t *testing.T) {
	for _, c := range oneSideCases() {
		t.Run(c.name, func(t *testing.T) { requireTheAdviceWithoutConflictMarkers(t, c) })
	}
}

func requireTheAdviceWithoutConflictMarkers(t *testing.T, c oneSideCase) {
	t.Helper()
	requireTheAdviceThenRunAgain(t, c.offer, c.want)
	if strings.Contains(c.offer.text, "with the conflict markers") {
		t.Errorf("offer %q; there are no conflict markers to make from one version", c.offer.text)
	}
}

func TestAFileOneSideDeletedWhoseNameCannotBePrintedGetsTheStepsWithoutTheName(t *testing.T) {
	offer := mergeConflictOffer("cfg/p\x1b.json", onlyTheirs, false)

	requireTheAdviceThenRunAgain(t, offer, "only one side of the conflict still has the file, so there are no conflict markers; keep the version of "+unprintedFile+
		" that the side being merged holds by bringing it back, mark it resolved, and commit, or "+acceptingTheDeletionOf(unprintedFile))
	if strings.Contains(offer.text, "\x1b") || strings.Contains(offer.text, "p.json") {
		t.Errorf("offer %q; want the unprintable name left out", offer.text)
	}
}

// Stage 2 is the side being merged into, stage 3 the side being merged, and stage 1 the base, which is no side.
func TestTheSidesThatHoldTheFileAreReadFromTheStagesOfTheIndex(t *testing.T) {
	entry := func(stage string) gitrepo.IndexEntry { return gitrepo.IndexEntry{Stage: stage} }
	cases := map[string]struct {
		entries []gitrepo.IndexEntry
		want    conflictSides
	}{
		"both modified":        {[]gitrepo.IndexEntry{entry("1"), entry("2"), entry("3")}, bothSides},
		"both added":           {[]gitrepo.IndexEntry{entry("2"), entry("3")}, bothSides},
		"deleted by the first": {[]gitrepo.IndexEntry{entry("1"), entry("3")}, onlyTheirs},
		"deleted by the other": {[]gitrepo.IndexEntry{entry("1"), entry("2")}, onlyOurs},
		"deleted by both":      {[]gitrepo.IndexEntry{entry("1")}, neitherSide},
		"not in conflict":      {[]gitrepo.IndexEntry{entry("0")}, neitherSide},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) { requireTheSides(t, c.entries, c.want) })
	}
}

func requireTheSides(t *testing.T, entries []gitrepo.IndexEntry, want conflictSides) {
	t.Helper()
	if got := conflictSidesOf(entries); got != want {
		t.Errorf("conflictSidesOf = %+v; want %+v", got, want)
	}
}
