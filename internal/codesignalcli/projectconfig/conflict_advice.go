package projectconfig

import (
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
// message about a missing policy describes. There is no file to edit, and git add on a path that is not there
// records a deletion and drops the other side, so the file is brought back first. When both sides hold a version,
// git checkout --merge recreates the conflict markers from them. When one side deleted the file it fails for want
// of the other version and there are no markers to resolve, so the advice is to keep the version that is left or
// to accept the deletion.
//
// A checkout of a pathspec is forced, and when an untracked file stands where a parent directory of the policy
// belongs, git unlinks the file to make the directory. So where a file is in the way, every checkout is behind the
// step that moves it. Accepting a deletion with git rm touches the index entry alone, and is not.
func resolveConflictAdvice(repoPath string, sides conflictSides, blocked bool) string {
	named := " (the name has a character that is not printed here)"
	if ValidatePath(repoPath) == nil && shellword.Printable(repoPath) {
		named = " -- " + shellword.Quote(":(top,literal)"+repoPath)
	}
	switch {
	case sides.ours && sides.theirs:
		return behindTheMove("bring the conflicted file back with git checkout --merge"+named, blocked) + ", resolve the conflict markers in it, and git add it"
	case sides.ours:
		return keepOrDelete("--ours", named, blocked)
	case sides.theirs:
		return keepOrDelete("--theirs", named, blocked)
	}
	return "both sides deleted it; accept the deletion with git rm" + named
}

// keepOrDelete is the advice for a file that only one side still holds: there are no conflict markers, so it is
// either kept as that side has it or deleted.
func keepOrDelete(side, named string, blocked bool) string {
	keep := behindTheMove("keep that side's version with git checkout "+side+named, blocked)
	return "only one side of the conflict still has the file, so there are no conflict markers; " + keep +
		" and git add it, or accept the deletion with git rm" + named
}

// behindTheMove is a step that writes the policy's path, put after the step that moves the file in the way of it.
func behindTheMove(step string, blocked bool) string {
	if blocked {
		return "move the file that blocks the path, then " + step
	}
	return step
}
