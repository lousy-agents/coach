package projectconfig

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// A restore of a path that carries a skip-worktree mark, or that a sparse checkout leaves out, is refused by git ("pathspec did not
// match"), so the restore that keeps a staged edit cannot run, and the one that drops it can, because it names the revision. If
// the staged edit is reported first, the only printed command that works is the one that discards the edit. The mark or the
// checkout is therefore named first, and the staged edit is mentioned in that offer.
func TestAMarkOrASparseCheckoutIsNamedBeforeAStagedEditBecauseItStopsTheRestoreThatKeepsTheEdit(t *testing.T) {
	regular := gitrepo.TreeEntry{Mode: "100644", SHA: priorityBlob}
	marked := []gitrepo.IndexEntry{indexEntryOf("100644", "0", true)}
	plain := []gitrepo.IndexEntry{indexEntryOf("100644", "0", false)}
	cases := []struct {
		name string
		got  restoreVerdict
		want restoreVerdict
	}{
		{"a staged edit and a skip-worktree mark", verdictFor(regular, true, indexFacts{entries: marked, unread: true}), restoreSkipWorktreeMarked},
		{"a staged edit and a sparse checkout that leaves the path out", verdictFor(regular, true, indexFacts{entries: marked, unread: true, sparse: true}), restoreSkippedBySparseCheckout},
		{"a staged edit and no mark", verdictFor(regular, true, indexFacts{entries: plain, unread: true}), restoreDiscardsStagedEdit},
		{"a skip-worktree mark and no staged edit", verdictFor(regular, true, indexFacts{entries: marked}), restoreSkipWorktreeMarked},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { requireTheVerdict(t, c.got, c.want) })
	}
}

func TestTheOfferForAMarkedPolicyMentionsTheStagedEditAndPrintsNoRestore(t *testing.T) {
	cases := []struct {
		name    string
		verdict restoreVerdict
		clear   string
	}{
		{"a skip-worktree mark", restoreSkipWorktreeMarked, "update-index --no-skip-worktree"},
		{"a sparse checkout", restoreSkippedBySparseCheckout, "add its directory to the sparse checkout"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { requireTheOfferToMentionTheStagedEdit(t, c.verdict, c.clear) })
	}
}

func requireTheOfferToMentionTheStagedEdit(t *testing.T, verdict restoreVerdict, clear string) {
	t.Helper()
	missing := &ConfigError{Kind: KindMissing, Missing: MissingAbsent}

	withEdit, _ := restoreOfferFor(policyJudgment{verdict: verdict, indexEntryUnread: true}, "HEAD", missing, "cfg/p.json")
	without, _ := restoreOfferFor(policyJudgment{verdict: verdict}, "HEAD", missing, "cfg/p.json")

	if !strings.Contains(withEdit.text, "the index also holds a staged edit to it") || !strings.Contains(withEdit.text, clear) {
		t.Errorf("offer %q; want it to say that the index holds a staged edit, and to name %q", withEdit.text, clear)
	}
	if strings.Contains(withEdit.text, "git restore") {
		t.Errorf("offer %q; a restore cannot run until the mark is cleared, and the one that can discards the edit", withEdit.text)
	}
	if strings.Contains(without.text, "staged edit") {
		t.Errorf("offer %q; it names a staged edit that the index does not hold", without.text)
	}
}
