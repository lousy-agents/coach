package projectconfig

import (
	"errors"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

func TestAnUncommittedPolicyWithAnUnresolvedConflictIsToldAsAConflictAndNotAsAFileToCommit(t *testing.T) {
	fake := indexFake{uncommitted: true, lsFiles: indexLine("100644", committedBlob, "1") + indexLine("100644", otherBlob, "3")}

	state, err := UncommittedPolicyInIndex(fake.run, "repo", "HEAD", uncommittedPolicyPath)

	if err != nil {
		t.Fatalf("UncommittedPolicyInIndex: %v", err)
	}
	requireWords(t, state.Message, "exists in the worktree", "unresolved merge conflict", "resolve the conflict in it, mark it resolved, and commit it", `"cfg/p.json"`, `"HEAD"`)
	// git refuses to commit while the path is unmerged, and the file is on disk, so the words name no command.
	requireNoWords(t, state.Message, "commit the file", "git ")
}

func TestAnUncommittedPolicyWhoseIndexEntryGitCannotListGivesNoAdvice(t *testing.T) {
	fake := indexFake{uncommitted: true, entryListingErr: errors.New("boom")}

	state, err := UncommittedPolicyInIndex(fake.run, "repo", "HEAD", uncommittedPolicyPath)

	if err == nil {
		t.Fatal("UncommittedPolicyInIndex returned no error for an index git could not list: the caller must report a failure of git, not a verdict about the policy")
	}
	if state.Cause != projectreadiness.CauseGitUnreadable || state.HeldByTheIndex {
		t.Errorf("cause %q, held %v; want git_unreadable and not held, because nothing is known of the index", state.Cause, state.HeldByTheIndex)
	}
	requireWords(t, state.Message, "Coach could not check what the index holds for it", "gives no advice")
	requireNoWords(t, state.Message, "commit the file", "resolve the conflict")
}

func TestAnUncommittedPolicyWhoseSubmoduleQuestionGitCannotAnswerStillGivesNoAdvice(t *testing.T) {
	fake := indexFake{uncommitted: true, submoduleListingErr: errors.New("boom")}

	state, err := UncommittedPolicyInIndex(fake.run, "repo", "HEAD", uncommittedPolicyPath)

	if err == nil || state.Cause != projectreadiness.CauseGitUnreadable || state.HeldByTheIndex {
		t.Errorf("UncommittedPolicyInIndex = cause %q, held %v, error %v; want git_unreadable, not held, and an error", state.Cause, state.HeldByTheIndex, err)
	}
}
