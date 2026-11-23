package projectcheck

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
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

// A question git cannot answer is an operational error for a policy that is gone from disk, where the answer decides
// which way out is safe: a gap would claim a cause for a state Coach never established. For a policy that is on disk the
// file is the policy, so the question is only one of how it relates to the revision, and the result says it could not
// be asked.
func TestAnUnansweredSubmoduleQuestionStopsTheCheckOnlyWhereTheFileIsGoneFromDisk(t *testing.T) {
	const policy = "cfg/p.json"
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	run := whereOnlyTheSubmoduleQuestionFails(repo)

	writeRegularFile(t, filepath.Join(repo, policy))
	onDisk, err := judgePolicy(run, repo, "HEAD", policyref.Named(policy))
	if err != nil || onDisk.check.State != projectreadiness.Pass || len(onDisk.warnings) != 1 || onDisk.warnings[0].Code != projectreadiness.WarnPolicyHeadUnreadable {
		t.Errorf("file on disk: judgePolicy = %+v, warnings %+v, %v; want a pass that says %q", onDisk.check, onDisk.warnings, err, projectreadiness.WarnPolicyHeadUnreadable)
	}

	failOnError(t, os.RemoveAll(filepath.Join(repo, "cfg")))
	gone, err := judgePolicy(run, repo, "HEAD", policyref.Named(policy))
	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) {
		t.Errorf("file gone from disk: judgePolicy = %+v, %v; want an *OperationalError for the unanswered question", gone.check, err)
	}
}
