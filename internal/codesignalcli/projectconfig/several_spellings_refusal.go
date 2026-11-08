package projectconfig

import (
	"fmt"
	"path"
	"slices"
	"sort"
	"strings"
)

// pathChooser says who chose the policy path being read. A path Coach found has one spelling it can be read at, so
// renaming the committed policy is the only way out; a path the user named is as likely a slip as a wish, and renaming
// the committed policy to match a slip breaks everyone who passes the committed spelling, so passing it is offered first.
type pathChooser int

const (
	chosenByCoach pathChooser = iota
	chosenByTheUser
)

// passTheCommittedSpelling is the way out that only a path the user named has, for the one spelling committed.
func (c pathChooser) passTheCommittedSpelling(committed string) string {
	if c != chosenByTheUser {
		return ""
	}
	return fmt.Sprintf("pass the committed spelling, %q, or ", committed)
}

// passOneOfTheCommittedSpellings is the same way out when several are committed.
func (c pathChooser) passOneOfTheCommittedSpellings() string {
	if c != chosenByTheUser {
		return ""
	}
	return "pass one of the committed spellings, or "
}

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

// directoryInCase is the sentence for a rename that changes a directory's name in case only: git cannot do that in one
// step on a disk that folds case, and it is empty where the directories are already spelled alike.
func directoryInCase(from, to string) string {
	if path.Dir(from) == path.Dir(to) {
		return ""
	}
	return " A directory whose name changes only in case cannot be renamed in one step on a disk that folds case: rename it to another name first, then to the one you want."
}

// recordTheRename is the advice for the policy committed under one other spelling: typed is the path Coach reads, and
// record the path git is to record the policy at.
func recordTheRename(committed, typed, record string, chooser pathChooser) string {
	return chooser.passTheCommittedSpelling(committed) + makeGitRecord(record) + thenRunAgain(typed, record) + "." + directoryInCase(committed, record) + " " + mindWhatStandsThere(record)
}

// severalSpellingsReason names every spelling the revision holds besides the one Coach reads, so that none stays
// committed unseen, and says how to keep one of them: it is renamed to the path Coach reads, and the others are
// deleted from the repository. With more spellings than are named, no spelling is chosen: it would leave one
// committed unseen.
func severalSpellingsReason(others []string, truncated bool, repoPath, record string, chooser pathChooser) string {
	listed := slices.Sorted(slices.Values(others))
	named := quotedList(listed)
	if truncated {
		named += ", and more"
	}
	reason := fmt.Sprintf("the analyzed revision holds the policy under several spellings, %s, and Coach reads %q: %skeep one of them at %s and remove the others", named, repoPath, chooser.passOneOfTheCommittedSpellings(), keptAt(repoPath, record))
	if truncated {
		return reason + thenRunAgain(repoPath, record) + ". " + mindWhatStandsThere(record)
	}
	keep := keptSpelling(others, record)
	rest := slices.DeleteFunc(slices.Clone(listed), func(spelling string) bool { return spelling == keep })
	return fmt.Sprintf("%s. To keep %q, %s, delete %s from the repository%s.%s %s", reason, keep, makeGitRecord(record), quotedList(rest), thenRunAgain(repoPath, record), directoryInCase(keep, record), mindWhatStandsThere(record))
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
