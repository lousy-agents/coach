package projectconfig

import (
	"errors"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

// workingCopy says whether a new document put at the path of a committed policy that cannot be used would lose
// work: whether the file, and the index entry for it, are the committed copy.
type workingCopy int

const (
	// workingCopyUntouched is a file that is the committed copy, or is gone, with an index that holds the committed
	// entry or nothing.
	workingCopyUntouched workingCopy = iota
	// workingCopyChanged is a file in the working tree that differs from the committed copy.
	workingCopyChanged
	// workingCopyStaged is an index entry that holds an edit.
	workingCopyStaged
	// workingCopyConflicted is an index entry that is unmerged.
	workingCopyConflicted
)

// settledByTheWorkingCopy is loaded, with the way out of a committed policy that cannot be used set to what the file
// can bear. A new document replaces the file, so it is offered only where the file is the committed copy and the index
// holds no edit and no conflict for it; where there is work in it, the error says so and names how to review it, and
// offers no replacement, in the scan and in the check alike. A question git cannot answer is told as such and leaves
// the replacement out: it is not read as a file with nothing in it.
func settledByTheWorkingCopy(run gitrepo.Runner, dir, revision, repoPath string, loaded error) error {
	var configErr *ConfigError
	if !errors.As(loaded, &configErr) || !configErr.ReplacesPolicy() {
		return loaded
	}
	state, err := inspectWorkingCopy(run, dir, revision, repoPath)
	if err != nil {
		return annotated(configErr, undecidableOffer(newGitFailure("whether the policy in the working tree differs from the committed one", err)))
	}
	if state == workingCopyUntouched {
		return loaded
	}
	return annotated(configErr, workingCopyOffer(state, revision, repoPath))
}

// inspectWorkingCopy reads the index entry of the policy and the file against the committed copy at revision. A
// conflict and a staged edit come first, because the file is judged against the index only when the index is the
// committed entry.
func inspectWorkingCopy(run gitrepo.Runner, dir, revision, repoPath string) (workingCopy, error) {
	committed, found, err := gitrepo.BlobEntryAtRevision(run, dir, revision, repoPath)
	if err != nil {
		return workingCopyUntouched, err
	}
	indexed, err := gitrepo.IndexEntries(run, dir, repoPath)
	if err != nil {
		return workingCopyUntouched, err
	}
	entries := withoutEmptyEntries(indexed.Entries)
	switch {
	case holdsUnmergedEntry(entries):
		return workingCopyConflicted, nil
	case indexHoldsUnreadEntry(committed, found, entries):
		return workingCopyStaged, nil
	}
	changed, err := differsOnDisk(run, dir, revision, repoPath)
	if changed {
		return workingCopyChanged, err
	}
	return workingCopyUntouched, err
}

// differsOnDisk is whether the file in the working tree differs from the copy committed at revision, which git says
// of the two as it does of any diff. A file that is gone has no work in it.
func differsOnDisk(run gitrepo.Runner, dir, revision, repoPath string) (bool, error) {
	exists, err := configExistsInWorktree(run, dir, repoPath)
	if err != nil || !exists {
		return false, err
	}
	names, err := run(dir, "diff", "--name-only", "--no-ext-diff", "--no-textconv", revision, "--", ":(top,literal)"+repoPath)
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(names)) != "", nil
}

// workingCopyOffer is the way out for a file with work in it: no replacement, and how to review what is there. A name
// that cannot be shown gets the step without the command.
func workingCopyOffer(state workingCopy, revision, repoPath string) restoreOffer {
	switch state {
	case workingCopyStaged:
		return restoreOffer{
			remedy: projectreadiness.RemedyNone,
			cause:  projectreadiness.CauseStagedEdit,
			text:   ". The index holds a staged edit to it, which a replacement would discard, so none is offered; " + reviewAdvice("git diff --cached -- ", repoPath, "review the staged edit") + ", and commit it if it is the fix",
		}
	case workingCopyConflicted:
		return restoreOffer{
			remedy: projectreadiness.RemedyNone,
			cause:  projectreadiness.CauseMergeConflict,
			text:   ". The index holds an unresolved merge conflict for it, and a replacement would mark it resolved with content from neither side, so none is offered; resolve the conflict markers in it, and git add it, then commit it",
		}
	}
	return restoreOffer{
		remedy: projectreadiness.RemedyNone,
		cause:  projectreadiness.CauseChangedOnDisk,
		text:   ". The copy in the working tree differs from the committed one, and a replacement would discard what is in it, so none is offered; " + reviewAdvice("git diff "+revision+" -- ", repoPath, "review it") + ", and commit it if it is the fix",
	}
}

// reviewAdvice names the command that shows the work in the file, with the path quoted and anchored at the top of the
// repository, or the step alone for a name that cannot be shown.
func reviewAdvice(command, repoPath, step string) string {
	if ValidatePath(repoPath) != nil || !shellword.Printable(repoPath) {
		return step + " (the name has a character that is not printed here)"
	}
	return step + " with " + command + shellword.Quote(":(top,literal)"+repoPath)
}
