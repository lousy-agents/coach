package projectconfig

import (
	"path"
	"slices"
	"sort"
	"strings"
)

// maxSpellings is how many spellings of one policy path a refusal names. A listing could hold any number of
// them, and a message that named all would not fit a terminal.
const maxSpellings = 16

// spellingSearch collects the paths of a revision whose every component equals the policy path's, ignoring case.
type spellingSearch struct {
	entries   revisionEntries
	found     []string
	truncated bool
}

// spellingsInRevision is every path of the revision that equals parts in every component ignoring case: all of
// them, where spellingInRevision follows the component spelled as typed whenever the revision has one. That is
// enough to say where a policy is, and not enough to say what else is committed under another case. It reports
// truncated when there are more than maxSpellings. A listing git could not give is the error.
func spellingsInRevision(entries revisionEntries, parts []string) (spellings []string, truncated bool, err error) {
	search := &spellingSearch{entries: entries}
	if err := search.below("", parts); err != nil {
		return nil, false, err
	}
	return search.found, search.truncated, nil
}

func (s *spellingSearch) below(directory string, parts []string) error {
	if len(parts) == 0 {
		s.add(directory)
		return nil
	}
	names, err := s.entries(directory)
	if err != nil {
		return err
	}
	for _, name := range names {
		if !strings.EqualFold(name, parts[0]) {
			continue
		}
		if err := s.below(path.Join(directory, name), parts[1:]); err != nil {
			return err
		}
	}
	return nil
}

func (s *spellingSearch) add(spelling string) {
	if len(s.found) == maxSpellings {
		s.truncated = true
		return
	}
	s.found = append(s.found, spelling)
}

// keptSpelling is the spelling to keep when several are committed: the one that needs the fewest components
// renamed to be the path Coach reads, and of those the one whose first difference is deepest, since a file
// can be renamed in the index with git mv where a directory cannot be renamed in case only.
func keptSpelling(spellings []string, repoPath string) string {
	typed := strings.Split(repoPath, "/")
	ranked := slices.Clone(spellings)
	sort.Slice(ranked, func(i, j int) bool {
		differing := func(spelling string) (count, first int) {
			first = len(typed)
			for index, part := range strings.Split(spelling, "/") {
				if part != typed[index] {
					count++
					first = min(first, index)
				}
			}
			return count, first
		}
		ci, fi := differing(ranked[i])
		cj, fj := differing(ranked[j])
		if ci != cj {
			return ci < cj
		}
		if fi != fj {
			return fi > fj
		}
		return ranked[i] < ranked[j]
	})
	return ranked[0]
}
