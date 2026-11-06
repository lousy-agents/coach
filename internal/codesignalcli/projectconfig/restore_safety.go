package projectconfig

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// restoreVerdict is what a restore of a committed policy that is missing from
// disk would do, judged before one is offered.
type restoreVerdict int

const (
	restoreNotCommitted restoreVerdict = iota
	restoreOverwritesNothing
	restoreDiscardsStagedEdit
	restoreRecreatesLink
	restoreSkippedBySparseCheckout
	restoreUndecidable
	restoreMergeConflict
	restoreStagedNotCommitted
	restoreSkipWorktreeMarked
	restoreUnmergedNotCommitted
	restoreStagedNonFileNotCommitted
	restoreStagedSkipWorktreeNotCommitted
	restoreStagedSparseNotCommitted
	restoreOverwritesEmptyEntry
	restoreDirectoryInIndex
	// restoreVerdictCount is the number of verdicts above, for a test that walks all of them.
	restoreVerdictCount
)

const regularFileModeMask = "100"

// policyJudgment is what git says about a policy path the working tree could not
// supply. The two facts are separate because the index can hold something nothing
// has read whatever a restore would do: a policy committed as a link, or never
// committed, can have a regular file staged over it.
type policyJudgment struct {
	// verdict is what a restore would do.
	verdict restoreVerdict
	// indexEntryUnread is set when the index holds an entry that differs from the
	// committed policy (any entry at all when nothing is committed), or git could
	// not say, so a report built without that entry does not reflect it.
	indexEntryUnread bool
	// failure is set when a git call that decides the verdict failed, so the
	// message can say which question went unanswered and what git said.
	failure *gitFailure
	// indexLocked is set when a restore command would otherwise be offered while a lock file stops git
	// from writing the index: running it would fail, so none is offered.
	indexLocked bool
}

// gitFailure is one git call that failed while a policy path was judged.
type gitFailure struct {
	// question is what the call was meant to settle, worded to follow "could not
	// check".
	question string
	err      error
}

// newGitFailure keeps git's own error. An OperationalError wraps it in Coach's
// message, which describe would otherwise quote back to the user inside
// Coach's own.
func newGitFailure(question string, err error) *gitFailure {
	var operational *gitrepo.OperationalError
	if errors.As(err, &operational) && operational.Cause != nil {
		err = operational.Cause
	}
	return &gitFailure{question: question, err: err}
}

// judgeRestore decides what a restore of repoPath would do and whether the index
// holds an entry nothing has read. It declines, naming why in the verdict, when
// the revision holds something other than a regular file, which the scan would
// refuse again after the restore; when the index holds an edit or an unmerged
// entry that the command would discard; and when a sparse checkout or a
// skip-worktree mark keeps git restore from writing the file. A staged deletion
// is not an edit a restore loses: it restores the committed blob. When nothing
// commits the path but the index holds a copy, the verdict says so, so a new
// policy is not offered over staged work; an unmerged copy is told apart, because
// git restore refuses it. When git cannot answer it declines
// too, so an operational failure is never taken for a clean miss. run is the git
// runner, a parameter so a caller's seam decides what git says.
func judgeRestore(run gitrepo.Runner, dir, revision, repoPath string) policyJudgment {
	committed, found, err := gitrepo.BlobEntryAtRevision(run, dir, revision, repoPath)
	if err != nil {
		return policyJudgment{verdict: restoreUndecidable, indexEntryUnread: true, failure: newGitFailure("whether it is committed at the analyzed revision", err)}
	}
	indexed, err := gitrepo.IndexEntries(run, dir, repoPath)
	if err != nil {
		return policyJudgment{verdict: verdictWithoutIndex(committed, found), indexEntryUnread: true, failure: newGitFailure("what the index holds for it", err)}
	}
	if indexed.HoldsBelow {
		return policyJudgment{verdict: restoreDirectoryInIndex}
	}
	listed := indexed.Entries
	staged := withoutUnmarkedEmptyEntries(listed)
	unread := indexHoldsUnreadEntry(committed, found, withoutEmptyEntries(listed))
	facts := indexFacts{entries: staged, unread: unread, sparse: sparseCheckoutEnabled(run, dir, staged), setAsideEmpty: len(staged) < len(listed)}
	verdict := verdictFor(committed, found, facts)
	return policyJudgment{verdict: verdict, indexEntryUnread: unread, indexLocked: offersRestoreCommand(verdict) && indexIsLocked(run, dir)}
}

// offersRestoreCommand is whether a verdict ends in a printed restore command, which is what a
// locked index makes fail.
func offersRestoreCommand(verdict restoreVerdict) bool {
	return verdict == restoreOverwritesNothing || verdict == restoreOverwritesEmptyEntry || verdict == restoreStagedNotCommitted
}

// indexIsLocked is whether the index lock file exists, which git leaves while another git process
// writes the index and after one died. Both forms of restore Coach prints write the index. Where git
// cannot say, the answer is no: the two reads before this one succeeded.
func indexIsLocked(run gitrepo.Runner, dir string) bool {
	output, err := run(dir, "rev-parse", "--git-path", "index.lock")
	if err != nil {
		return false
	}
	lock := strings.TrimSpace(string(output))
	if lock == "" {
		return false
	}
	if !filepath.IsAbs(lock) {
		lock = filepath.Join(dir, lock)
	}
	_, statErr := os.Lstat(lock)
	return statErr == nil
}

// sparseCheckoutEnabled reports whether core.sparseCheckout is on. It asks git
// only when a skip-worktree entry makes the answer matter, and an unset key,
// which git reports with a failure exit, is taken as off.
func sparseCheckoutEnabled(run gitrepo.Runner, dir string, staged []gitrepo.IndexEntry) bool {
	if !skippedBySparseCheckout(staged) {
		return false
	}
	out, err := run(dir, "config", "--type=bool", "--get", "core.sparseCheckout")
	return err == nil && strings.TrimSpace(string(out)) == "true"
}
