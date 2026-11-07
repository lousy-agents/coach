package projectcheck

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

const packageManifestBlob = "100644 blob 0123456789012345678901234567890123456789\tpackage.json\x00"

// reporter is the part of *testing.T the git fakes need to hold a test to what it set out to do.
type reporter interface {
	Helper()
	Cleanup(func())
	Errorf(format string, args ...any)
}

// questionsWhereOnly names the one git question that fails, answers package.json's tree entry and the check that the
// revision is a commit. Any other question is an error naming it, so a question that production starts to ask fails the test that did not
// expect it, and does not succeed with empty output.
//
// The test also fails when the question it named was never asked. A fake that answers everything else lets the
// code stop at an earlier step for another reason and the test pass without ever reaching the step it is about.
func questionsWhereOnly(t reporter, failing func(args []string) bool) func(string, ...string) ([]byte, error) {
	t.Helper()
	asked := false
	t.Cleanup(func() {
		if !asked {
			t.Errorf("questionsWhereOnly: the question that was to fail was never asked, so the code stopped at another step")
		}
	})
	return func(_ string, args ...string) ([]byte, error) {
		switch {
		case failing(args):
			asked = true
			return nil, errNoAnswer
		case args[0] == "ls-tree" && strings.HasSuffix(args[len(args)-1], "package.json"):
			return []byte(packageManifestBlob), nil
		case args[0] == "cat-file":
			return nil, nil
		case theRevisionsDirectoryListing(args):
			return nil, nil
		}
		return nil, fmt.Errorf("questionsWhereOnly: unexpected git question: %s", strings.Join(args, " "))
	}
}

// gitAnswer is what a fake git says to one question: its output, or its failure.
type gitAnswer struct {
	output string
	err    error
}

// reportQuestionsNeverAsked fails the test once for each listed question that was not asked.
func reportQuestionsNeverAsked(t reporter, answers map[string]gitAnswer, asked map[string]bool) {
	for question := range answers {
		if !asked[question] {
			t.Errorf("questionsAnsweredExactly: %q was never asked, so the code did not take the path the test is about", question)
		}
	}
}

// questionsAnsweredExactly answers each listed question, matched by its whole argument list, and fails any other
// question with an error that names it. It answers no question by its kind: a revision check for another
// revision, or a tree lookup of another path, is not a listed question. The test also fails when a listed
// question was never asked, so the list is the sequence the code under test asks and a test cannot pass by
// stopping at a question before the one that fails.
func questionsAnsweredExactly(t reporter, answers map[string]gitAnswer) func(string, ...string) ([]byte, error) {
	t.Helper()
	asked := map[string]bool{}
	t.Cleanup(func() { reportQuestionsNeverAsked(t, answers, asked) })
	return func(_ string, args ...string) ([]byte, error) {
		question := strings.Join(args, " ")
		answer, listed := answers[question]
		if !listed {
			return nil, fmt.Errorf("questionsAnsweredExactly: unexpected git question: %s", question)
		}
		asked[question] = true
		return []byte(answer.output), answer.err
	}
}

func theRepositoryRootLookup(args []string) bool {
	return args[0] == "rev-parse" && len(args) > 1 && args[1] == "--show-toplevel"
}

// theRevisionsDirectoryListing is the question whether the revision commits a policy path in another case, which lists
// a directory of the tree and finds it empty.
func theRevisionsDirectoryListing(args []string) bool {
	return args[0] == "ls-tree" && slices.Contains(args, "--name-only")
}

func theTreeLookupOfThePolicy(args []string) bool {
	return args[0] == "ls-tree" && !theRevisionsDirectoryListing(args) && !strings.HasSuffix(args[len(args)-1], "package.json")
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
	withReadinessGit(t, questionsWhereOnly(t, theRevisionCheck))

	check, _, err := checkPolicy("repo", "HEAD", "cfg/p.json")

	requireOperationalError(t, check, err)
}

func TestARepositoryRootThatCannotBeFoundStopsTheNamedPolicyCheck(t *testing.T) {
	withReadinessGit(t, questionsWhereOnly(t, theRepositoryRootLookup))

	check, _, err := checkExplicitPolicy(gitfixture.Init(t), "HEAD", "cfg/p.json")

	if !errors.Is(err, errNoAnswer) {
		t.Errorf("check = %+v, error %v; want the failed root lookup returned as it is", check, err)
	}
}

// A committed policy that git cannot look up stops the relevance of the dirty worktree, and the error says
// that the lookup of the policy failed. The questions are listed: the revision check and package.json's tree
// entry that come first, the policy's own lookup that fails, and nothing else is answered. A fake that
// failed an earlier question would stop the report there and the test would say nothing of the policy.
func TestACommittedPolicyThatCannotBeLookedUpStopsTheRelevanceOfTheDirtyWorktree(t *testing.T) {
	const manifestBlob = "0123456789012345678901234567890123456789"
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	withReadinessGit(t, questionsAnsweredExactly(t, map[string]gitAnswer{
		"cat-file -e HEAD^{commit}":                                 {},
		"ls-tree -z --full-tree HEAD -- :(top,literal)package.json": {output: packageManifestBlob},
		"cat-file -e " + manifestBlob:                               {},
		"ls-tree -z --full-tree HEAD -- :(top,literal)cfg/p.json":   {err: errNoAnswer},
	}))
	refused := func(string, string, string) (projectreadiness.Check, []string, error) {
		return projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPolicyInvalid}, nil, nil
	}

	result, err := evaluate(repo, "HEAD", policyref.Named("cfg/p.json"), refused)

	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) {
		t.Fatalf("evaluate = %+v, %v; want the lookup of the committed policy to stop it with an *OperationalError", result.Status, err)
	}
	if !errors.Is(operational.Cause, errNoAnswer) || !strings.Contains(operational.Message, `"cfg/p.json" could not be read at revision "HEAD"`) {
		t.Errorf("error = %q (cause %v); want it to say that the policy could not be read, from the failed lookup", operational.Message, operational.Cause)
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

// RunAfterPolicyError reports the verdict the caller holds for a named policy and reads the rest of the
// project itself. A question about the rest that git cannot answer fails the report: a clean report with
// nothing in it is not an answer. The policy check asks git nothing here, so the one question that fails is
// the first one after it.
func TestANamedPolicyReportFailsWhenAQuestionAfterThePolicyCheckCannotBeAnswered(t *testing.T) {
	withReadinessGit(t, questionsWhereOnly(t, theRevisionCheck))
	held := &projectconfig.ConfigError{Kind: projectconfig.KindMissing, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseParentNotDirectory, FromWorkingTree: true}

	result, err := RunAfterPolicyError(gitfixture.Init(t), "HEAD", policyref.Named("cfg/p.json"), held)

	var operational *gitrepo.OperationalError
	if !errors.As(err, &operational) || result != nil {
		t.Errorf("RunAfterPolicyError = %+v, %v; want no report and the operational error", result, err)
	}
}
