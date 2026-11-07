package projectconfig

import (
	"fmt"
	"path"
	"strings"
)

// maxSpellings is how many spellings of one policy path a refusal names. A listing could hold any number of
// them, and a message that named all would not fit a terminal.
const maxSpellings = 16

// spellingSearch collects the paths of a revision whose every component equals the policy path's, ignoring case.
type spellingSearch struct {
	entries   revisionEntries
	typed     string
	others    []string
	held      bool
	truncated bool
}

// spellingsInRevision is every path of the revision that equals parts in every component ignoring case, apart from
// the path as typed: those are the others, and held says whether the revision commits the path as typed as well. It
// reports truncated when there are more than maxSpellings others. A listing git could not give is the error.
func spellingsInRevision(entries revisionEntries, parts []string) (others []string, held, truncated bool, err error) {
	search := &spellingSearch{entries: entries, typed: strings.Join(parts, "/")}
	if err := search.below("", parts); err != nil {
		return nil, false, false, err
	}
	return search.others, search.held, search.truncated, nil
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
	switch {
	case spelling == s.typed:
		s.held = true
	case len(s.others) == maxSpellings:
		s.truncated = true
	default:
		s.others = append(s.others, spelling)
	}
}

func quotedList(items []string) string {
	quoted := make([]string, len(items))
	for i, item := range items {
		quoted[i] = fmt.Sprintf("%q", item)
	}
	return strings.Join(quoted, ", ")
}
