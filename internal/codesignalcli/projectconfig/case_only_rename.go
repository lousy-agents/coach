package projectconfig

import (
	"fmt"
	"path"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

// caseOnlyRenameRefusal is the refusal for a policy that is on disk but not committed under that
// name, when the analyzed revision holds the same path in another case and git ignores case. It is nil
// otherwise, which leaves the policy plainly uncommitted. The user changed only the case of a name, and on
// such a repository `git add` records nothing for that, so "commit the file" would end in "nothing to
// commit". The refusal is nil as well when the index already records the typed name (the rename is staged, and
// the commit that records it is the way out), and the failure is a question git could not answer, which is not
// "plainly uncommitted" either.
//
// What it returns is nil, a *ConfigError for the refusal, or a *gitFailure for the question that went
// unanswered; a caller tells them apart with errors.As.
func caseOnlyRenameRefusal(run gitrepo.Runner, dir, revision, repoPath string) error {
	return caseOnlyRenameRefusalWith(run, dir, entriesThrough(run, dir, revision), repoPath)
}

func caseOnlyRenameRefusalWith(run gitrepo.Runner, dir string, entries revisionEntries, repoPath string) error {
	ignores, err := gitrepo.IgnoresCase(run, dir)
	if err != nil {
		return newGitFailure("whether git ignores case", err)
	}
	if !ignores {
		return nil
	}
	return unlessTheRenameIsStaged(run, dir, repoPath, caseOnlyRename(entries, repoPath, moveGuardFor(run, dir)))
}

func caseOnlyRename(entries revisionEntries, repoPath string, mayMove moveGuard) error {
	parts := strings.Split(repoPath, "/")
	spelled, ambiguous, changed, err := spellingInRevision(entries, parts)
	if err != nil {
		return newGitFailure(listingQuestion, err)
	}
	if ambiguous != nil || !changed {
		return nil
	}
	held := strings.Join(spelled, "/")
	reason := fmt.Sprintf("exists on disk, but the analyzed revision holds the path as %q, and git is set to ignore case (core.ignorecase), so git add records nothing for the difference%s", held, renameAdvice(parts, spelled, mayMove))
	return newUnusableOnDisk(repoPath, projectreadiness.CauseNameCaseDiffers, reason)
}

// renameAdvice follows from the first component that differs. A file can be renamed in the index with
// git mv, which records a change of case where git add does not; a directory cannot, because git mv
// refuses to rename a directory in case only, so it is renamed back.
func renameAdvice(typed, held []string, mayMove moveGuard) string {
	index := firstDifference(typed, held)
	pass := passClause(" and pass", strings.Join(held, "/"))
	if index < len(typed)-1 {
		return fmt.Sprintf("; rename %q back to %q%s", path.Join(typed[:index+1]...), path.Join(held[:index+1]...), pass)
	}
	from, to := strings.Join(held, "/"), strings.Join(typed, "/")
	command, ok := gitMoveCommand(from, to)
	if !ok {
		return fmt.Sprintf("; rename %q back to %q%s", to, from, pass)
	}
	if !mayMove(from, to) {
		return fmt.Sprintf("; a different file stands at %q (the committed file is %q), and git mv could overwrite it, so no command is printed: move your file aside, or rename the committed one yourself, then run again", to, from)
	}
	return fmt.Sprintf("; record the rename with %s and commit it, or rename the file back%s", command, pass)
}

func firstDifference(a, b []string) int {
	for i := range a {
		if a[i] != b[i] {
			return i
		}
	}
	return len(a)
}

// atTheRepositoryRoot is the git a command is run with so that it works from any directory. The paths Coach
// prints are repository-relative, like --project-config, and git mv reads its paths against the working
// directory, so the same command fails from a subdirectory with "bad source". If the root cannot be found
// the -C is empty, which leaves git where it is, and the move fails there for want of a repository.
const atTheRepositoryRoot = `git -C "$(git rev-parse --show-toplevel)"`

// gitMoveCommand is the command that moves a tracked file to the name on disk and so records the
// rename. It reports false for a name that cannot be shown to a terminal as it is.
func gitMoveCommand(from, to string) (string, bool) {
	if !movable(from, to) {
		return "", false
	}
	return atTheRepositoryRoot + " mv -- " + shellword.Quote(from) + " " + shellword.Quote(to), true
}

func movable(from, to string) bool {
	return ValidatePath(from) == nil && ValidatePath(to) == nil && shellword.Printable(from) && shellword.Printable(to)
}
