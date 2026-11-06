package projectconfig

import (
	"errors"
	"os/exec"
	"slices"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// repositoryWithASubmodule records a submodule at vendor/sub in the index, which is all the probe reads.
func repositoryWithASubmodule(t *testing.T) string {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	head, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	if err := exec.Command("git", "-C", repo, "update-index", "--add", "--cacheinfo", "160000,"+string(head[:40])+",vendor/sub").Run(); err != nil {
		t.Fatal(err)
	}
	return repo
}

// failingEach is a runner that answers with real git except for the subcommands it is told to fail.
func failingEach(failures map[string]error) gitrepo.Runner {
	return func(dir string, args ...string) ([]byte, error) {
		if failure, fails := failures[args[0]+" "+firstFlag(args)]; fails {
			return nil, failure
		}
		return gitrepo.RunBytes(dir, args...)
	}
}

func firstFlag(args []string) string {
	if len(args) > 1 {
		return args[1]
	}
	return ""
}

// The probe is one more git read of the policy, so it goes through the runner the caller holds to its
// limits, like every other: a test that fakes that runner controls it.
func TestTheSubmoduleProbeAsksTheRunnerItWasGiven(t *testing.T) {
	repo := repositoryWithASubmodule(t)
	var asked []string
	run := func(dir string, args ...string) ([]byte, error) {
		asked = append(asked, args[0])
		return gitrepo.RunBytes(dir, args...)
	}

	message, cause := UncommittedPolicy(run, repo, "HEAD", "vendor/sub/coach.json")

	if cause != projectreadiness.PolicyCause("in_submodule") || !strings.Contains(message, `inside the submodule "vendor/sub"`) {
		t.Errorf("UncommittedPolicy = %q, %q; want the submodule and its cause", message, cause)
	}
	if !slices.Contains(asked, "ls-files") {
		t.Errorf("the runner was asked %v; the probe never reached it", asked)
	}
}

// When git cannot say whether the path lies inside a submodule, the message gives no advice: "commit
// the file" is the circular advice for a file in a submodule, and a guess is no reason to give it.
func TestAnUnansweredSubmoduleQuestionGivesNoAdvice(t *testing.T) {
	repo := repositoryWithASubmodule(t)
	run := failingEach(map[string]error{"ls-files --stage": errors.New("boom")})

	message, cause := UncommittedPolicy(run, repo, "HEAD", "vendor/sub/coach.json")

	if strings.Contains(message, "commit the file") || cause != projectreadiness.PolicyCause("git_unreadable") {
		t.Errorf("UncommittedPolicy = %q, %q; want no advice and the cause git_unreadable", message, cause)
	}
}

// A path inside a submodule that is missing is not offered a new policy either: a file written there
// is refused as uncommittable, so authoring leads nowhere.
func TestAMissingPolicyInsideASubmoduleIsOfferedNeitherARestoreNorANewPolicy(t *testing.T) {
	repo := repositoryWithASubmodule(t)

	status, err := ExplicitOnDiskStatus(gitrepo.RunBytes, repo, "HEAD", "vendor/sub/cfg/p.json")

	if err != nil || status == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want a missing-policy error", status, err)
	}
	if status.Remedy != projectreadiness.RemedyNone || status.PolicyCause != projectreadiness.PolicyCause("in_submodule") || !strings.Contains(status.Message, `inside the submodule "vendor/sub"`) {
		t.Errorf("remedy %d, cause %q, message %q; want no remedy, in_submodule, and the submodule named", status.Remedy, status.PolicyCause, status.Message)
	}
}

// A file whose existence cannot be checked is not called absent: "was not found" would offer a new
// policy over a file that may be there.
func TestAPolicyThatCannotBeLocatedIsNotCalledAbsent(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	run := failingEach(map[string]error{"rev-parse --show-toplevel": errors.New("boom")})

	err := readError(run, repo, "HEAD", "cfg.json", errors.New("not at the revision"))

	found, ok := err.(*ConfigError)
	if !ok || found.Remedy != projectreadiness.RemedyNone || found.PolicyCause != projectreadiness.PolicyCause("git_unreadable") || strings.Contains(found.Message, "was not found") {
		t.Errorf("readError = %+v; want no remedy, the cause git_unreadable, and not \"was not found\"", err)
	}
}

// A submodule is one entry, so an ancestor whose listing overruns the output budget is a directory with
// a great many files in it. That answers the question (no); it is not a failure that stops the policy
// from being judged.
func TestAnAncestorTooLargeToListIsADirectoryNotASubmodule(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	overrun := func(string, ...string) ([]byte, error) { return nil, &gitrepo.BoundError{Kind: gitrepo.BoundStdout} }

	submodule, err := enclosingSubmodule(overrun, repo, "src/cfg/p.json")

	if err != nil || submodule != "" {
		t.Errorf("enclosingSubmodule = %q, %v; want none and no error", submodule, err)
	}
}
