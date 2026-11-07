package projectconfig

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// gitThatHoldsOnlyCfgPolicy is the whole of git for a repository whose revision holds cfg/p.json and nothing else,
// that is not a submodule anywhere, and that ignores case when ignoreCase says so. A question it was not given is an
// error that names it. The case-only rename is a state of a case-folding file system, but what the refusal makes of
// it is decided from these three answers, so it is specified here, where any file system can run it.
func gitThatHoldsOnlyCfgPolicy(ignoreCase string) func(string, ...string) ([]byte, error) {
	return func(_ string, args ...string) ([]byte, error) {
		switch {
		case args[0] == "ls-files":
			return nil, nil
		case args[0] == "config":
			return []byte(ignoreCase + "\n"), nil
		case args[0] == "ls-tree" && slices.Contains(args, "--name-only"):
			return namesUnder(args[len(args)-1]), nil
		}
		return nil, errors.New("gitThatHoldsOnlyCfgPolicy: unexpected git question: " + strings.Join(args, " "))
	}
}

// namesUnder is the listing of the directory a pathspec like :(top,literal)cfg/ names, the root when it is not one.
func namesUnder(last string) []byte {
	directory := strings.TrimSuffix(strings.TrimPrefix(last, ":(top,literal)"), "/")
	if directory == "cfg" {
		return []byte("p.json\x00")
	}
	return []byte("cfg\x00")
}

// A policy on disk under a name that differs only by case from what the revision holds, on a repository that
// ignores case, is told apart from one that is only not committed: git add records nothing for the difference.
func TestAnUncommittedPolicyInAnotherCaseThanTheRevisionHoldsHasTheCaseCause(t *testing.T) {
	message, cause, unanswered := UncommittedPolicy(gitThatHoldsOnlyCfgPolicy("true"), "repo", "HEAD", "cfg/P.json")

	if unanswered != nil || cause != projectreadiness.CauseNameCaseDiffers {
		t.Errorf("UncommittedPolicy = %q, %q, %v; want the cause %q", message, cause, unanswered, projectreadiness.CauseNameCaseDiffers)
	}
	if !strings.Contains(message, `holds the path as "cfg/p.json"`) {
		t.Errorf("message %q; want it to name the spelling the revision holds", message)
	}
}

func TestAnUncommittedPolicyIsPlainlyUncommittedWhereGitTellsCasesApart(t *testing.T) {
	_, cause, unanswered := UncommittedPolicy(gitThatHoldsOnlyCfgPolicy("false"), "repo", "HEAD", "cfg/P.json")

	if unanswered != nil || cause != projectreadiness.CauseNotCommitted {
		t.Errorf("UncommittedPolicy cause %q, %v; want %q, since cfg/P.json is another file there", cause, unanswered, projectreadiness.CauseNotCommitted)
	}
}
