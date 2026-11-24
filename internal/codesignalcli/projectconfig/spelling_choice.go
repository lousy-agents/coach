package projectconfig

import (
	"path"
	"slices"
	"sort"
	"strings"
)

// pathChooser says who chose the policy path being read: a path Coach found is read with --project-config at another
// spelling, and a path the user named is read by passing that spelling instead.
type pathChooser int

const (
	chosenByCoach pathChooser = iota
	chosenByTheUser
)

// directoryInCase is the sentence for a rename that changes a directory's name in case only: git cannot do that in one
// step on a disk that folds case, and it is empty where the directories are already spelled alike.
func directoryInCase(from, to string) string {
	if path.Dir(from) == path.Dir(to) {
		return ""
	}
	return " A directory whose name changes only in case cannot be renamed in one step on a disk that folds case: rename it to another name first, then to the one you want."
}

// keptSpelling is the spelling to keep when the disk lists several: the one that needs the fewest components
// renamed to be the path Coach reads, and of those the one whose first difference is deepest, since a file can be
// renamed in one step where a directory whose name changes only in case cannot on a disk that folds case.
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
