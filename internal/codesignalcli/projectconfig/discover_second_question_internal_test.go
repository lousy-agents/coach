package projectconfig

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// aRepositoryWhoseSecondTreeEntryQuestionFails is the whole of git for discovery in a repository with a commit
// and nothing at the policy's paths, where the first question about the entry at the canonical path (is there a
// blob) is answered and the second (is there something else) fails, as a second identical read can after a
// first that fit its time, and every later one is answered again, so that a failure dropped has nothing else to
// stop the scan. Any question it was not given is an error that names it.
func aRepositoryWhoseSecondTreeEntryQuestionFails(repo string, failure error) func(string, ...string) ([]byte, error) {
	entryQuestions := 0
	return func(_ string, args ...string) ([]byte, error) {
		switch {
		case args[0] == "rev-parse":
			return []byte(repo + "\n"), nil
		case args[0] == "cat-file":
			return nil, nil
		case args[0] == "ls-tree" && slices.Contains(args, "--name-only"):
			return nil, nil
		case args[0] == "ls-tree":
			entryQuestions++
			if entryQuestions == 2 {
				return nil, failure
			}
			return nil, nil
		case args[0] == "show":
			return nil, errors.New("fatal: path does not exist")
		}
		return nil, errors.New("aRepositoryWhoseSecondTreeEntryQuestionFails: unexpected git question: " + strings.Join(args, " "))
	}
}

// Discovery asks two questions about the entry at the canonical path: whether it is a blob, and when it is not,
// whether it is something a policy cannot be. A failure of the second is not an answer of "nothing there": the scan
// would go on without a policy, or fall back to the legacy one, over a path it could not read.
func TestDiscoverFailsWhenOnlyTheSecondQuestionAboutTheEntryCannotBeAnswered(t *testing.T) {
	repo := committedRepository(t)
	original := runProjectConfigGit
	t.Cleanup(func() { runProjectConfigGit = original })
	runProjectConfigGit = aRepositoryWhoseSecondTreeEntryQuestionFails(repo, errBoom)

	selection, err := Discover(repo, "HEAD")

	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) || selection.Fatal != nil || selection.Path != "" {
		t.Errorf("Discover = %+v, %v; want an *OperationalError and no policy", selection, err)
	}
}
