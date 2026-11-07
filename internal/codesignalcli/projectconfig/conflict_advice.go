package projectconfig

import (
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

// conflictSides says which sides of a merge conflict still hold the policy. The index holds a version for each
// of them: stage 2 for the side being merged into, stage 3 for the side being merged, and stage 1 for the base,
// which is no side. A side that deleted the file has no version.
type conflictSides struct {
	ours, theirs bool
}

// conflictSidesOf reads the sides from the index entries of the policy.
func conflictSidesOf(entries []gitrepo.IndexEntry) conflictSides {
	var sides conflictSides
	for _, entry := range entries {
		sides.ours = sides.ours || entry.Stage == "2"
		sides.theirs = sides.theirs || entry.Stage == "3"
	}
	return sides
}

// resolveConflictAdvice is how to resolve a conflict in a policy that is missing from disk, which is the state a
// message about a missing policy describes, told in words: Coach names the file and the step, and the user carries it
// out. There is no file to edit, and marking a path that is not there as resolved records a deletion and drops the
// other side, so the file is brought back first. When both sides hold a version, git can make the conflict markers from
// them. When one side deleted the file there is only one version and no markers to resolve, so the advice is to keep
// the version that is left or to accept the deletion.
//
// Bringing a file back is forced, and when an untracked file stands where a parent directory of the policy belongs,
// git unlinks the file to make the directory. So where a file is in the way, every step that brings the file back is
// behind the step that moves it. Accepting a deletion touches the index entry alone, and is not.
func resolveConflictAdvice(repoPath string, sides conflictSides, blocked bool) string {
	file := conflictedFileNamed(repoPath)
	switch {
	case sides.ours && sides.theirs:
		return behindTheMove("bring back "+file+" with the conflict markers git makes from the two sides", blocked) + ", resolve the conflict in it, mark it resolved, and commit"
	case sides.ours:
		return keepOrDelete("the side being merged into", file, blocked)
	case sides.theirs:
		return keepOrDelete("the side being merged", file, blocked)
	}
	return "both sides deleted it; " + acceptTheDeletion(file)
}

// conflictedFileNamed is how the advice refers to the policy: its path, quoted, or the file without its name when the
// name has a character that would not be shown safely.
func conflictedFileNamed(repoPath string) string {
	if ValidatePath(repoPath) == nil && shellword.Printable(repoPath) {
		return fmt.Sprintf("%q", repoPath)
	}
	return "the conflicted file (the name has a character that is not printed here)"
}

// keepOrDelete is the advice for a file that only one side still holds: there are no conflict markers, so it is
// either kept as that side has it or deleted.
func keepOrDelete(side, file string, blocked bool) string {
	keep := behindTheMove("keep the version of "+file+" that "+side+" holds by bringing it back", blocked)
	return "only one side of the conflict still has the file, so there are no conflict markers; " + keep +
		", mark it resolved, and commit, or " + acceptTheDeletion(file)
}

func acceptTheDeletion(file string) string {
	return "accept the deletion by recording " + file + " as deleted and committing"
}

// behindTheMove is a step that writes the policy's path, put after the step that moves the file in the way of it.
func behindTheMove(step string, blocked bool) string {
	if blocked {
		return "move the file that blocks the path, then " + step
	}
	return step
}
