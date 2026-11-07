package projectconfig

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/shellword"
)

// severalSpellingsReason names every spelling the revision holds, so that none stays committed unseen, and
// prints the commands that keep one of them: it is moved to the path Coach reads, and the others leave the
// index. They leave the index only because where a file system folds case they name the file just moved. With
// more spellings than are named, or one that cannot be shown to a terminal, no command is printed: it would
// leave a spelling committed or print a name it cannot show.
func severalSpellingsReason(spellings []string, truncated bool, repoPath string, mayMove moveGuard) string {
	listed := slices.Sorted(slices.Values(spellings))
	named := quotedList(listed)
	if truncated {
		named += ", and more"
	}
	reason := fmt.Sprintf("the analyzed revision holds the policy under several spellings, %s, and Coach reads %q: keep one of them at that path and remove the others", named, repoPath)
	keep := keptSpelling(spellings, repoPath)
	move, movable := moveCommand(keep, repoPath)
	others := slices.DeleteFunc(slices.Clone(listed), func(spelling string) bool { return spelling == keep })
	if truncated || !movable {
		return reason
	}
	if !mayMove(keep, repoPath) {
		return fmt.Sprintf("%s; a different file stands at %q, and a move could overwrite it, so no command is printed: move that file aside first", reason, repoPath)
	}
	quoted := make([]string, len(others))
	for i, other := range others {
		quoted[i] = shellword.Quote(other)
	}
	return fmt.Sprintf("%s; to keep %q, run %s && git -C \"$top\" --literal-pathspecs rm --cached -q -- %s and commit", reason, keep, move, strings.Join(quoted, " "))
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
