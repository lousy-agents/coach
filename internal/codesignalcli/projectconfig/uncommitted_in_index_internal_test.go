package projectconfig

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

const uncommittedPolicyPath = "cfg/p.json"

type uncommittedInIndexCase struct {
	name      string
	fake      indexFake
	wantCause projectreadiness.PolicyCause
	wantHeld  bool
}

func uncommittedInIndexCases() []uncommittedInIndexCase {
	return []uncommittedInIndexCase{
		{"the index knows nothing of it", indexFake{uncommitted: true}, projectreadiness.CauseNotCommitted, false},
		{"a staged copy", indexFake{uncommitted: true, lsFiles: indexLine("100644", otherBlob, "0")}, projectreadiness.CauseNotCommitted, true},
		{"a staged copy marked skip-worktree", indexFake{uncommitted: true, lsFiles: skippedIndexLine("100644", otherBlob, "0")}, projectreadiness.CauseNotCommitted, true},
		{"an entry that holds no content, in a SHA-1 repository", indexFake{uncommitted: true, lsFiles: indexLine("100644", emptyBlobSHA1, "0")}, projectreadiness.CauseNotCommitted, false},
		{"an entry that holds no content, in a SHA-256 repository", indexFake{uncommitted: true, lsFiles: indexLine("100644", emptyBlobSHA256, "0")}, projectreadiness.CauseNotCommitted, false},
		{"a conflict one side deleted", indexFake{uncommitted: true, lsFiles: indexLine("100644", committedBlob, "1") + indexLine("100644", otherBlob, "3")}, projectreadiness.CauseMergeConflict, true},
		{"a conflict both sides changed", indexFake{uncommitted: true, lsFiles: indexLine("100644", committedBlob, "1") + indexLine("100644", committedBlob, "2") + indexLine("100644", otherBlob, "3")}, projectreadiness.CauseMergeConflict, true},
		{"a conflict that holds only the side being merged into", indexFake{uncommitted: true, lsFiles: indexLine("100644", otherBlob, "2")}, projectreadiness.CauseMergeConflict, true},
	}
}

func TestAnUncommittedPolicyOnDiskIsHeldByTheIndexWhenTheIndexHoldsWorkForIt(t *testing.T) {
	for _, c := range uncommittedInIndexCases() {
		t.Run(c.name, func(t *testing.T) { assertHeldAndCause(t, c) })
	}
}

func assertHeldAndCause(t *testing.T, c uncommittedInIndexCase) {
	t.Helper()
	state, err := UncommittedPolicyInIndex(c.fake.run, "repo", "HEAD", uncommittedPolicyPath)

	if err != nil || state.Cause != c.wantCause || state.HeldByTheIndex != c.wantHeld {
		t.Errorf("UncommittedPolicyInIndex = cause %q, held %v, error %v; want cause %q, held %v", state.Cause, state.HeldByTheIndex, err, c.wantCause, c.wantHeld)
	}
}

func TestAnUncommittedPolicyTheIndexHoldsNothingForKeepsTheWordsForAFileToCommit(t *testing.T) {
	for _, fake := range []indexFake{
		{uncommitted: true},
		{uncommitted: true, lsFiles: indexLine("100644", otherBlob, "0")},
		{uncommitted: true, lsFiles: indexLine("100644", emptyBlobSHA1, "0")},
	} {
		want, _, wantErr := UncommittedPolicy(fake.run, "repo", "HEAD", uncommittedPolicyPath)
		state, err := UncommittedPolicyInIndex(fake.run, "repo", "HEAD", uncommittedPolicyPath)

		if wantErr != nil || err != nil || state.Message != want {
			t.Errorf("UncommittedPolicyInIndex(%q) = %q, %v; want the words of UncommittedPolicy, %q", fake.lsFiles, state.Message, err, want)
		}
	}
}
