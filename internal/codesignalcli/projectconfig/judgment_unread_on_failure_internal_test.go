package projectconfig

import "testing"

// A question git cannot answer after the index was read leaves an index entry that nothing has read, and the
// judgment keeps saying so. The dirty-worktree section lists the policy as a path the report did not read only
// while it does, and a policy that is listed as read is taken for the file on disk.
func TestAQuestionGitCannotAnswerAfterTheIndexWasReadLeavesTheEntryUnread(t *testing.T) {
	cases := map[string]indexFake{
		"whether a sparse checkout is on, with a skip-worktree entry that holds an edit": {configErr: errBoom, lsFiles: skippedIndexLine("100644", otherBlob, "0")},
		"whether the index is locked, with a staged copy of a policy nothing commits":    {uncommitted: true, gitPathErr: errBoom, lsFiles: indexLine("100644", otherBlob, "0")},
		"whether the path lies inside a submodule":                                       {submoduleListingErr: errBoom},
	}
	for question, fake := range cases {
		t.Run(question, func(t *testing.T) { requireUndecidableAndUnread(t, fake) })
	}
}

func requireUndecidableAndUnread(t *testing.T, fake indexFake) {
	t.Helper()

	judged := judgeRestore(fake.run, "repo", "HEAD", "pol.json")

	if judged.verdict != restoreUndecidable || !judged.indexEntryUnread || judged.failure == nil {
		t.Errorf("judgeRestore = verdict %d, unread %v, failure %v; want undecidable, unread, and the failed question named", judged.verdict, judged.indexEntryUnread, judged.failure)
	}
}

// core.sparseCheckout matters only to an entry marked skip-worktree, so it is not asked for any other, and a
// setting that cannot be read does not fail a judgment it cannot change.
func TestTheSparseSettingIsNotAskedWhenNoEntryIsMarkedSkipWorktree(t *testing.T) {
	fake := indexFake{configErr: errBoom, lsFiles: indexLine("100644", committedBlob, "0")}

	judged := judgeRestore(fake.run, "repo", "HEAD", "pol.json")

	if judged.verdict != restoreOverwritesNothing || judged.failure != nil {
		t.Errorf("judgeRestore = verdict %d, failure %v; want a clean restore: no entry is marked skip-worktree, so the setting is not asked", judged.verdict, judged.failure)
	}
}

// The note that a new policy may need git add --sparse is advice, and it is not given on a guess: when git cannot
// say where the repository is, the answer is no, even where core.sparseCheckout is on.
func TestTheSparseNoteNeedsTheRepositoryRoot(t *testing.T) {
	fake := indexFake{sparse: true, rootErr: errBoom}

	if newPolicyMayNeedSparse(fake.run, "repo", "cfg/p.json") {
		t.Error("newPolicyMayNeedSparse = true; want false when the repository root cannot be found")
	}
}
