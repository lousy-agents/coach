package projectconfig

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

const priorityBlob = "0123456789012345678901234567890123456789"

func indexEntryOf(mode, stage string, skipWorktree bool) gitrepo.IndexEntry {
	return gitrepo.IndexEntry{TreeEntry: gitrepo.TreeEntry{Mode: mode, SHA: priorityBlob}, Stage: stage, SkipWorktree: skipWorktree}
}

// Where two states hold at once, the verdict names the one the comments on verdictFor and
// verdictForUncommitted put first. Each row holds two states that an earlier arm and a later arm both match,
// so swapping the arms changes the verdict.
func TestAVerdictNamesTheStateThatComesFirstWhenTwoHold(t *testing.T) {
	link := gitrepo.TreeEntry{Mode: "120000", SHA: priorityBlob}
	regular := gitrepo.TreeEntry{Mode: "100644", SHA: priorityBlob}
	cases := []struct {
		name string
		got  restoreVerdict
		want restoreVerdict
	}{
		{"a committed link that is also unmerged: the link, which a restore would recreate",
			verdictFor(link, true, indexFacts{entries: []gitrepo.IndexEntry{indexEntryOf("100644", "2", false)}}), restoreRecreatesLink},
		{"a skip-worktree mark beside an empty entry that was set aside: the mark, which alone stops a restore",
			verdictFor(regular, true, indexFacts{entries: []gitrepo.IndexEntry{indexEntryOf("100644", "0", true)}, setAsideEmpty: true}), restoreSkipWorktreeMarked},
		{"a staged link that is also marked skip-worktree and nothing commits: the link, which cannot be a policy",
			verdictForUncommitted([]gitrepo.IndexEntry{indexEntryOf("120000", "0", true)}, true), restoreStagedNonFileNotCommitted},
		{"an unmerged copy that is also marked skip-worktree and nothing commits: the conflict, which git restore refuses",
			verdictForUncommitted([]gitrepo.IndexEntry{indexEntryOf("100644", "2", true)}, true), restoreUnmergedNotCommitted},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { requireTheVerdict(t, c.got, c.want) })
	}
}

func requireTheVerdict(t *testing.T, got, want restoreVerdict) {
	t.Helper()
	if got != want {
		t.Errorf("verdict = %d; want %d", got, want)
	}
}

// A policy inside a submodule is told so before the index is mentioned: nothing there can be restored or
// committed in this repository, whatever a lock on the index would say.
func TestAPolicyInsideASubmoduleIsToldSoBeforeTheIndexIsMentioned(t *testing.T) {
	judgment := policyJudgment{verdict: restoreOverwritesNothing, submodule: "vendor/sub", indexLocked: true}

	offer, offered := restoreOfferFor(judgment, "HEAD", &ConfigError{Kind: KindMissing, Missing: MissingAbsent}, "vendor/sub/p.json")

	if !offered || offer.cause != "in_submodule" {
		t.Errorf("offer = %+v, %v; want the submodule's, which comes before every other question", offer, offered)
	}
}
