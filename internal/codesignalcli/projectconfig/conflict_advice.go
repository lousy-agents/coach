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
func resolveConflictAdvice(repoPath string, sides conflictSides) string {
	named := " (the name has a character that is not printed here)"
	if ValidatePath(repoPath) == nil && shellword.Printable(repoPath) {
		named = " -- " + shellword.Quote(":(top,literal)"+repoPath)
	}
	switch {
	case sides.ours && sides.theirs:
		return "bring the conflicted file back with git checkout --merge" + named + ", resolve the conflict markers in it, and git add it"
	case sides.ours:
		return keepOrDelete("--ours", named)
	case sides.theirs:
		return keepOrDelete("--theirs", named)
	}
	return "both sides deleted it; accept the deletion with git rm" + named
}

// keepOrDelete is the advice for a file that only one side still holds: there are no conflict markers, so it is
// either kept as that side has it or deleted.
func keepOrDelete(side, named string) string {
	return "only one side of the conflict still has the file, so there are no conflict markers; keep that side's version with git checkout " + side +
		named + " and git add it, or accept the deletion with git rm" + named
}
