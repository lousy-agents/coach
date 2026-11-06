package projectcheck

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// whereOnlyTheSubmoduleQuestionFails answers like a repository with a commit and nothing committed at the
// policy path, and fails the index listing that asks whether a path lies inside a submodule.
func whereOnlyTheSubmoduleQuestionFails(repo string) func(string, ...string) ([]byte, error) {
	return func(_ string, args ...string) ([]byte, error) {
		switch args[0] {
		case "ls-files":
			return nil, errNoAnswer
		case "rev-parse":
			return []byte(repo + "\n"), nil
		}
		return nil, nil
	}
}

// A question git cannot answer is an operational error in --check-project, whichever state the policy is
// in: a gap would claim a cause for a state Coach never established, and an exit code that depended on
// whether the file is on disk would make the same failure mean two things.
func TestAnUnansweredSubmoduleQuestionIsOneOperationalErrorWhetherOrNotTheFileIsOnDisk(t *testing.T) {
	const policy = "cfg/p.json"
	rows := map[string]func(t *testing.T, repo string){
		"the file is on disk and not committed": func(t *testing.T, repo string) { writeRegularFile(t, filepath.Join(repo, policy)) },
		"the file is gone from disk":            func(t *testing.T, repo string) { failOnError(t, os.RemoveAll(filepath.Join(repo, "cfg"))) },
	}
	for name, arrange := range rows {
		t.Run(name, func(t *testing.T) { requireOperationalWhenTheSubmoduleQuestionFails(t, policy, arrange) })
	}
}

func requireOperationalWhenTheSubmoduleQuestionFails(t *testing.T, policy string, arrange func(t *testing.T, repo string)) {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	arrange(t, repo)
	original := runReadinessGit
	t.Cleanup(func() { runReadinessGit = original })
	runReadinessGit = whereOnlyTheSubmoduleQuestionFails(repo)

	check, _, err := checkExplicitPolicy(repo, "HEAD", policy)

	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) {
		t.Errorf("checkExplicitPolicy = %+v, %v; want an *OperationalError for the unanswered question", check, err)
	}
}
