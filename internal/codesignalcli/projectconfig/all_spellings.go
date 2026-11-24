package projectconfig

import (
	"fmt"
	"path"
	"slices"
	"strings"
)

// maxSpellings is how many spellings of one policy path a refusal names. A listing could hold any number of
// them, and a message that named all would not fit a terminal.
const maxSpellings = 16

// spellingSearch collects the paths whose every component equals the policy path's, ignoring case.
type spellingSearch struct {
	entries directoryEntries
	typed   string
	others  []string
	held    bool
}

// spellingsListed is every listed path that equals parts in every component ignoring case, apart from the path as
// typed: those are the others, in order, and held says whether the path as typed is listed as well. It names the first maxSpellings others and reports truncated when there are more. A listing that could not be
// given is the error.
func spellingsListed(entries directoryEntries, parts []string) (others []string, held, truncated bool, err error) {
	search := &spellingSearch{entries: entries, typed: strings.Join(parts, "/")}
	if err := search.below("", parts); err != nil {
		return nil, false, false, err
	}
	others = slices.Sorted(slices.Values(search.others))
	if len(others) > maxSpellings {
		return others[:maxSpellings], search.held, true, nil
	}
	return others, search.held, false, nil
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

// add records one spelling. Every spelling is kept so that the ones named past maxSpellings are the first in order and
// not the first a listing happens to return.
func (s *spellingSearch) add(spelling string) {
	if spelling == s.typed {
		s.held = true
		return
	}
	s.others = append(s.others, spelling)
}

func quotedList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("%q", item)
	}
	return strings.Join(quoted, ", ")
}
