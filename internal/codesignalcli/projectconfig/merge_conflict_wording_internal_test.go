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

// A policy in a merge conflict is missing from disk (that is the state the message describes), so there is no file
// to edit, and git add on a path that is not there records a deletion and drops the other side. The message
// brings the file back with its conflict markers first.
func TestTheMergeConflictOffersBringTheConflictedFileBackBeforeAskingForItToBeResolved(t *testing.T) {
	offers := map[string]restoreOffer{
		"over a committed policy":      mergeConflictOffer("cfg/p.json", bothSides),
		"for a policy nothing commits": unmergedNotCommittedOffer("cfg/p.json", bothSides),
	}
	for name, offer := range offers {
		t.Run(name, func(t *testing.T) { requireTheFileIsBroughtBack(t, offer) })
	}
}

func requireTheFileIsBroughtBack(t *testing.T, offer restoreOffer) {
	t.Helper()
	const command = "git checkout --merge -- ':(top,literal)cfg/p.json'"
	if !strings.Contains(offer.text, command) || !strings.Contains(offer.text, "resolve the conflict markers in it, and git add it") {
		t.Errorf("offer %q; want the command %q and then the resolution", offer.text, command)
	}
	if strings.Contains(offer.text, "edit the file") || strings.Contains(offer.text, "git status") {
		t.Errorf("offer %q; the file is gone, so it cannot be edited, and git status resolves nothing", offer.text)
	}
}

func TestAConflictedNameThatCannotBePrintedGetsTheStepWithoutTheCommand(t *testing.T) {
	offer := mergeConflictOffer("cfg/p\x1b.json", bothSides)

	if strings.Contains(offer.text, "\x1b") || !strings.Contains(offer.text, "git checkout --merge (the name has a character that is not printed here)") {
		t.Errorf("offer %q; want the step named and the unprintable name left out", offer.text)
	}
}

// oneSideCase is an offer for a conflict that one side deleted the file in, and the advice it is to give.
type oneSideCase struct {
	name  string
	offer restoreOffer
	want  string
}

func oneSideCases() []oneSideCase {
	const keepAndDelete = ", or accept the deletion with git rm -- ':(top,literal)cfg/p.json'"
	sidesAndAdvice := map[string]struct {
		sides conflictSides
		want  string
	}{
		"only the merged side holds it":  {onlyTheirs, "git checkout --theirs -- ':(top,literal)cfg/p.json' and git add it" + keepAndDelete},
		"only the current side holds it": {onlyOurs, "git checkout --ours -- ':(top,literal)cfg/p.json' and git add it" + keepAndDelete},
		"neither side holds it":          {neitherSide, "both sides deleted it; accept the deletion with git rm -- ':(top,literal)cfg/p.json'"},
	}
	var cases []oneSideCase
	for name, c := range sidesAndAdvice {
		cases = append(cases,
			oneSideCase{name + " over a committed policy", mergeConflictOffer("cfg/p.json", c.sides), c.want},
			oneSideCase{name + " for a policy nothing commits", unmergedNotCommittedOffer("cfg/p.json", c.sides), c.want})
	}
	return cases
}

// git checkout --merge makes conflict markers from the versions of both sides and fails when one has none, and then
// there is nothing to resolve. The advice for a file one side deleted keeps the version that is left or deletes the
// file, and names the checkout flag of the side that still has it.
func TestAFileOneSideDeletedGetsTheCommandsThatWorkAndNotCheckoutMerge(t *testing.T) {
	for _, c := range oneSideCases() {
		t.Run(c.name, func(t *testing.T) { requireTheAdviceAndNotCheckoutMerge(t, c) })
	}
}

func requireTheAdviceAndNotCheckoutMerge(t *testing.T, c oneSideCase) {
	t.Helper()
	if !strings.Contains(c.offer.text, c.want) {
		t.Errorf("offer %q; want %q", c.offer.text, c.want)
	}
	if strings.Contains(c.offer.text, "checkout --merge") {
		t.Errorf("offer %q; git checkout --merge fails for want of the other version", c.offer.text)
	}
}

func TestAFileOneSideDeletedWhoseNameCannotBePrintedGetsTheStepsWithoutTheName(t *testing.T) {
	offer := mergeConflictOffer("cfg/p\x1b.json", onlyTheirs)

	const unprinted = " (the name has a character that is not printed here)"
	if strings.Contains(offer.text, "\x1b") || !strings.Contains(offer.text, "git checkout --theirs"+unprinted+" and git add it, or accept the deletion with git rm"+unprinted) {
		t.Errorf("offer %q; want the steps named and the unprintable name left out", offer.text)
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
