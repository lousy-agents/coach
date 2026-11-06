package projectcheck

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

const packageManifestBlob = "100644 blob 0123456789012345678901234567890123456789\tpackage.json\n"

// questionsWhereOnly names the one git question that fails; every other question has an explicit answer
// that lets a check go on: the revision resolves and package.json is committed.
func questionsWhereOnly(failing func(args []string) bool) func(string, ...string) ([]byte, error) {
	return func(_ string, args ...string) ([]byte, error) {
		switch {
		case failing(args):
			return nil, errNoAnswer
		case args[0] == "ls-tree" && strings.HasSuffix(args[len(args)-1], "package.json"):
			return []byte(packageManifestBlob), nil
		}
		return nil, nil
	}
}

func theRepositoryRootLookup(args []string) bool {
	return args[0] == "rev-parse" && len(args) > 1 && args[1] == "--show-toplevel"
}

func theTreeLookupOfThePolicy(args []string) bool {
	return args[0] == "ls-tree" && !strings.HasSuffix(args[len(args)-1], "package.json")
}

func theRevisionCheck(args []string) bool {
	return args[0] == "cat-file"
}

func withReadinessGit(t *testing.T, run func(string, ...string) ([]byte, error)) {
	t.Helper()
	original := runReadinessGit
	t.Cleanup(func() { runReadinessGit = original })
	runReadinessGit = run
}

func requireOperationalError(t *testing.T, got projectreadiness.Check, err error) {
	t.Helper()
	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) {
		t.Errorf("check = %+v, error %v; want an *OperationalError for the unanswered question", got, err)
	}
}

// A question git cannot answer stops the check at the step that asked it. Each step is failed alone, with
// every other question answered, so a step that dropped its error would go on with a zero value and report a
// clean miss.
func TestTheRevisionThatCannotBeVerifiedStopsTheDiscoveredPolicyCheck(t *testing.T) {
	withReadinessGit(t, questionsWhereOnly(theRevisionCheck))

	check, _, err := checkPolicy("repo", "HEAD", "cfg/p.json")

	requireOperationalError(t, check, err)
}

func TestARepositoryRootThatCannotBeFoundStopsTheNamedPolicyCheck(t *testing.T) {
	withReadinessGit(t, questionsWhereOnly(theRepositoryRootLookup))

	check, _, err := checkExplicitPolicy(gitfixture.Init(t), "HEAD", "cfg/p.json")

	if !errors.Is(err, errNoAnswer) {
		t.Errorf("check = %+v, error %v; want the failed root lookup returned as it is", check, err)
	}
}

func TestACommittedPolicyThatCannotBeLookedUpStopsTheRelevanceOfTheDirtyWorktree(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	withReadinessGit(t, questionsWhereOnly(theTreeLookupOfThePolicy))
	refused := func(string, string, string) (projectreadiness.Check, []string, error) {
		return projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyInvalid}, nil, nil
	}

	result, err := evaluate(repo, "HEAD", policyref.Named("cfg/p.json"), refused)

	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) {
		t.Errorf("evaluate = %+v, %v; want the lookup of the committed copy to stop it", result.Status, err)
	}
}

// Discovery asks git where the repository is. A directory that is not a repository has no policy to select,
// and the question's failure is returned and not turned into the canonical path.
func TestADiscoveredPolicyCannotBeSelectedOutsideARepository(t *testing.T) {
	path, warnings, err := resolveCheckPolicy(t.TempDir(), "HEAD", policyref.Discovered(""))

	if err == nil {
		t.Errorf("resolveCheckPolicy = %q, %v, nil; want the failed question returned", path, warnings)
	}
}
