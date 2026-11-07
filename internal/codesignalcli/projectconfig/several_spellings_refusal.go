package projectconfig

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// The advice for a policy committed under other spellings is a rename, told in words and never as a command: a pasted
// rename can overwrite an untracked file. It reads the same whether the rename is yet to be made or already staged,
// because git records it only when it is committed, and it warns of what a rename can overwrite whatever the disk or
// git's settings are, because no probe of either is needed to say so.

// makeGitRecord is the step the revision needs, for a rename made now or staged already.
func makeGitRecord(repoPath string) string {
	return fmt.Sprintf("make git record it as %q (rename it in the repository, or commit a rename you have staged)", repoPath)
}

// mindWhatStandsThere is the warning for a rename to repoPath: with core.ignorecase set, git skips its check that the
// destination exists for a rename that changes only the case of a name, and renames over a file there.
func mindWhatStandsThere(repoPath string) string {
	return fmt.Sprintf("If another file already exists at %q, move it aside first; with core.ignorecase set, git can overwrite it", repoPath)
}

// recordTheRename is the advice for the policy committed under one other spelling.
func recordTheRename(repoPath string) string {
	return makeGitRecord(repoPath) + ", then commit and run again. " + mindWhatStandsThere(repoPath)
}

// severalSpellingsReason names every spelling the revision holds besides the one Coach reads, so that none stays
// committed unseen, and says how to keep one of them: it is renamed to the path Coach reads, and the others are
// deleted from the repository. With more spellings than are named, no spelling is chosen: it would leave one
// committed unseen.
func severalSpellingsReason(others []string, truncated bool, repoPath string) string {
	listed := slices.Sorted(slices.Values(others))
	named := quotedList(listed)
	if truncated {
		named += ", and more"
	}
	reason := fmt.Sprintf("the analyzed revision holds the policy under several spellings, %s, and Coach reads %q: keep one of them at that path and remove the others", named, repoPath)
	if truncated {
		return reason + ", then commit and run again. " + mindWhatStandsThere(repoPath)
	}
	keep := keptSpelling(others, repoPath)
	rest := slices.DeleteFunc(slices.Clone(listed), func(spelling string) bool { return spelling == keep })
	return fmt.Sprintf("%s. To keep %q, %s, delete %s from the repository, then commit and run again. %s", reason, keep, makeGitRecord(repoPath), quotedList(rest), mindWhatStandsThere(repoPath))
}

// keptSpelling is the spelling to keep when several are committed: the one that needs the fewest components
// renamed to be the path Coach reads, and of those the one whose first difference is deepest, since a file
// can be renamed in the index with git mv where a directory cannot be renamed in case only.
func keptSpelling(spellings []string, repoPath string) string {
	typed := strings.Split(repoPath, "/")
	ranked := slices.Clone(spellings)
	sort.Slice(ranked, func(i, j int) bool { return renamesLess(ranked[i], ranked[j], typed) })
	return ranked[0]
}

func renamesLess(a, b string, typed []string) bool {
	countA, firstA := renamesNeeded(a, typed)
	countB, firstB := renamesNeeded(b, typed)
	if countA != countB {
		return countA < countB
	}
	if firstA != firstB {
		return firstA > firstB
	}
	return a < b
}

// renamesNeeded is how many components of spelling differ from typed, and the index of the first one.
func renamesNeeded(spelling string, typed []string) (count, first int) {
	first = len(typed)
	for index, part := range strings.Split(spelling, "/") {
		if part != typed[index] {
			count++
			first = min(first, index)
		}
	}
	return count, first
}
