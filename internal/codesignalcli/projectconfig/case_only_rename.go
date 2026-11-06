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
// commit".
func caseOnlyRenameRefusal(run gitrepo.Runner, dir, revision, repoPath string) *ConfigError {
	return caseOnlyRenameRefusalWith(run, dir, entriesThrough(run, dir, revision), repoPath)
}

func caseOnlyRenameRefusalWith(run gitrepo.Runner, dir string, entries revisionEntries, repoPath string) *ConfigError {
	if !gitrepo.IgnoresCase(run, dir) {
		return nil
	}
	return caseOnlyRename(entries, repoPath)
}

func caseOnlyRename(entries revisionEntries, repoPath string) *ConfigError {
	parts := strings.Split(repoPath, "/")
	spelled, ambiguous, changed := spellingInRevision(entries, parts)
	if ambiguous != nil || !changed {
		return nil
	}
	held := strings.Join(spelled, "/")
	reason := fmt.Sprintf("exists on disk, but the analyzed revision holds the path as %q, and this file system ignores case, so git add records nothing for the difference%s", held, renameAdvice(parts, spelled))
	return newUnusableOnDisk(repoPath, projectreadiness.CauseNameCaseDiffers, reason)
}

// renameAdvice follows from the first component that differs. A file can be renamed in the index with
// git mv, which records a change of case where git add does not; a directory cannot, because git mv
// refuses to rename a directory in case only, so it is renamed back.
func renameAdvice(typed, held []string) string {
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

// gitMoveCommand is the command that moves a tracked file to the name on disk and so records the
// rename. It reports false for a name that cannot be shown to a terminal as it is.
func gitMoveCommand(from, to string) (string, bool) {
	if ValidatePath(from) != nil || ValidatePath(to) != nil || !shellword.Printable(from) || !shellword.Printable(to) {
		return "", false
	}
	return "git mv " + shellword.Quote(from) + " " + shellword.Quote(to), true
}
