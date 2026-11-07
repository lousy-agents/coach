package projectcheck

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

// aRevisionWhoseSecondTreeEntryQuestionFails answers the first question about the entry at a path (is there a blob)
// and fails the second (is there something else there). Any other question is an error that names it.
func aRevisionWhoseSecondTreeEntryQuestionFails() func(string, ...string) ([]byte, error) {
	entryQuestions := 0
	return func(_ string, args ...string) ([]byte, error) {
		switch {
		case args[0] == "cat-file":
			return nil, nil
		case args[0] == "ls-tree" && slices.Contains(args, "-z"):
			entryQuestions++
			if entryQuestions == 2 {
				return nil, errNoAnswer
			}
			return nil, nil
		}
		return nil, errors.New("aRevisionWhoseSecondTreeEntryQuestionFails: unexpected git question: " + strings.Join(args, " "))
	}
}

// A path with no blob is missing only if nothing else is there. When the second question cannot be answered, the
// check stops with that failure, whatever the first said: it does not offer a new policy over a directory.
func TestAPathIsNotCalledMissingWhenOnlyTheSecondQuestionAboutItsEntryFails(t *testing.T) {
	withReadinessGit(t, aRevisionWhoseSecondTreeEntryQuestionFails())

	check, _, err := checkPolicy("repo", "HEAD", nonFilePolicy)

	requireOperationalError(t, check, err)
}
