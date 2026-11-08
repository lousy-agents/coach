package projectconfig

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

type stagedLinkVerdictCase struct {
	name  string
	facts indexFacts
	want  restoreVerdict
}

// stagedLinkVerdictCases are the states a committed policy with something staged over it can be in. Where two hold, the
// verdict is the one that names the step that has to come first: a conflict, a sparse checkout and a skip-worktree mark
// before a link, and a link before a plain staged edit.
func stagedLinkVerdictCases() []stagedLinkVerdictCase {
	link := []gitrepo.IndexEntry{indexEntryOf("120000", "0", false)}
	return []stagedLinkVerdictCase{
		{"a symbolic link staged over it", indexFacts{entries: link, unread: true}, restoreStagedNonFileOverCommitted},
		{"a submodule entry staged over it", indexFacts{entries: []gitrepo.IndexEntry{indexEntryOf("160000", "0", false)}, unread: true}, restoreStagedNonFileOverCommitted},
		{"a regular file staged over it", indexFacts{entries: []gitrepo.IndexEntry{indexEntryOf("100644", "0", false)}, unread: true}, restoreDiscardsStagedEdit},
		{"a link staged over it and marked skip-worktree", indexFacts{entries: []gitrepo.IndexEntry{indexEntryOf("120000", "0", true)}, unread: true}, restoreSkipWorktreeMarked},
		{"a link staged over it and a sparse checkout that leaves it out", indexFacts{entries: []gitrepo.IndexEntry{indexEntryOf("120000", "0", true)}, unread: true, sparse: true}, restoreSkippedBySparseCheckout},
		{"a link on one side of a conflict", indexFacts{entries: []gitrepo.IndexEntry{indexEntryOf("100644", "2", false), indexEntryOf("120000", "3", false)}, unread: true}, restoreMergeConflict},
	}
}

// A committed policy with a symbolic link (or another entry that is not a regular file) staged over it is a staged
// edit whose keep step recreates what the scan refuses. The verdict is told apart from a plain staged edit.
func TestAStagedLinkOverACommittedPolicyIsToldApartFromAStagedEdit(t *testing.T) {
	for _, c := range stagedLinkVerdictCases() {
		t.Run(c.name, func(t *testing.T) { assertTheVerdictOfTheStagedLinkCase(t, c) })
	}
}

func assertTheVerdictOfTheStagedLinkCase(t *testing.T, c stagedLinkVerdictCase) {
	t.Helper()
	committed := gitrepo.TreeEntry{Mode: "100644", SHA: committedBlob}

	if got := verdictFor(committed, true, c.facts); got != c.want {
		t.Errorf("verdictFor = %d; want %d", got, c.want)
	}
}

func TestTheOfferForAStagedLinkOverACommittedPolicyPrintsOnlyTheRestoreThatDropsIt(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		t.Run(fmt.Sprintf("a file in the way = %v", blocked), func(t *testing.T) { assertTheDropIsTheOnlyRestore(t, blocked) })
	}
}

func assertTheDropIsTheOnlyRestore(t *testing.T, blocked bool) {
	t.Helper()
	text := stagedNonFileOverCommittedOffer("HEAD", "cfg/p.json", blocked).text

	// the restore with no source writes the link back, which the scan refuses
	requireNoWords(t, text, "git restore -- ")
	requireWords(t, text, "git restore --source=HEAD --staged --worktree -- ':(top,literal)cfg/p.json'")
	if moved := strings.Contains(text, "move the file that blocks the path, then"); moved != blocked {
		t.Errorf("offer %q; a file in the way is moved before the restore, and only then", text)
	}
}

// Reviewing the staged entry changes nothing, so a step to run again after only that brings the same message back. The one
// step that ends in running again is the one that changes the index: staging a regular file where the link is.
func TestTheOnlyStepToRunAgainAfterIsTheOneThatStagesARegularFile(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		clauses := clausesAskingToRunAgain(stagedNonFileOverCommittedOffer("HEAD", "cfg/p.json", blocked).text)

		if len(clauses) != 1 || !strings.Contains(clauses[0], "stage a regular file at the path in place of the link") {
			t.Errorf("a file in the way = %v: the clauses that ask to run again are %q; want exactly one, and it stages a regular file", blocked, clauses)
		}
	}
}

func clausesAskingToRunAgain(text string) []string {
	var found []string
	for _, clause := range strings.Split(text, "; ") {
		if strings.Contains(clause, "run again") {
			found = append(found, clause)
		}
	}
	return found
}
