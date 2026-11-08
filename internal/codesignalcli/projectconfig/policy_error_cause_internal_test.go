package projectconfig

import (
	"errors"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

var errBoom = errors.New("git failed")

// Every error a policy path can produce names a remedy or says why it names none: never both, and
// never neither. The constructors set the remedy and the stable word side by side, so this holds
// each of them to it, including the three kinds of read that trips a bound, where a word beside
// "write a new policy" or no word beside "no way out" is easy to introduce.
func TestEveryPolicyErrorNamesARemedyOrACauseNeverBothAndNeverNeither(t *testing.T) {
	uncommitted := gitfixture.Init(t)
	gitfixture.CommitFile(t, uncommitted, "a.txt", "x")
	writeFile(t, uncommitted, "cfg.json", "{}")
	absent := gitfixture.Init(t)
	gitfixture.CommitFile(t, absent, "a.txt", "x")
	cases := map[string]error{
		"nothing at the path":                              missingError("cfg/p.json"),
		"a file where a parent should be":                  blockedParentError("cfg/p.json", "cfg"),
		"a path the working tree cannot use":               unusableOnDiskError("cfg/p.json", projectreadiness.PolicyCause("not_a_regular_file"), "is not a regular file"),
		"content the revision holds that is invalid":       invalidError("cfg/p.json", "HEAD", "document is not valid JSON"),
		"a file on disk the revision does not hold":        readError(NewGitRunner(), uncommitted, "HEAD", "cfg.json", errBoom),
		"a file nowhere":                                   readError(NewGitRunner(), absent, "HEAD", "cfg.json", errBoom),
		"a read over the output budget":                    readError(NewGitRunner(), absent, "HEAD", "cfg.json", &gitrepo.BoundError{Kind: gitrepo.BoundStdout}),
		"a read past the time bound":                       readError(NewGitRunner(), absent, "HEAD", "cfg.json", &gitrepo.BoundError{Kind: gitrepo.BoundTimeout}),
		"a read past the stderr bound":                     readError(NewGitRunner(), absent, "HEAD", "cfg.json", &gitrepo.BoundError{Kind: gitrepo.BoundStderr}),
		"a read git could not answer":                      unansweredRead("cfg/p.json", "HEAD", newGitFailure("what the revision holds at the path", errBoom)),
		"an entry the revision commits that is not a file": committedAsADirectory(t),
		"a policy committed in another case":               committedInAnotherCase(t),
		"a revision whose spellings git could not list":    WrongCaseAtRevision(failingGit, "repo", "HEAD", CanonicalPath),
	}
	for name, err := range cases {
		t.Run(name, func(t *testing.T) { requireTheErrorNamesOneOfThem(t, err) })
	}
}

func failingGit(string, ...string) ([]byte, error) { return nil, errBoom }

func committedAsADirectory(t *testing.T) error {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
	refusal, err := NonFileAtRevision(NewGitRunner(), repo, "HEAD", "cfg")
	if err != nil || refusal == nil {
		t.Fatalf("NonFileAtRevision = %v, %v; want the refusal for a directory", refusal, err)
	}
	return refusal
}

func committedInAnotherCase(t *testing.T) error {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, ".Lousy-Agents/Coach.json", "{}")
	refusal := WrongCaseAtRevision(NewGitRunner(), repo, "HEAD", CanonicalPath)
	if refusal == nil {
		t.Fatalf("WrongCaseAtRevision = nil; want the refusal for a policy committed in another case")
	}
	return refusal
}

// A path that cannot name a policy is refused as a usage error before any report exists, so it is the one
// error that names neither: it has no word in the cause vocabulary, and a gap made from it says undetermined
// (see projectcheck).
func TestAPathThatCannotNameAPolicyNamesNeitherARemedyNorACause(t *testing.T) {
	found, ok := invalidPathError("../p.json", "HEAD", "path must be normalized").(*ConfigError)

	if !ok || found.Remedy != projectreadiness.RemedyNone || found.PolicyCause != "" {
		t.Errorf("error = %+v; want a *ConfigError with no remedy and no cause", found)
	}
}

func requireTheErrorNamesOneOfThem(t *testing.T, err error) {
	t.Helper()
	found, ok := err.(*ConfigError)
	if !ok {
		t.Fatalf("error = %T, want a *ConfigError", err)
	}
	namesARemedy := found.Remedy != projectreadiness.RemedyNone
	if namesARemedy == (found.PolicyCause != "") {
		t.Errorf("remedy %d and cause %q in %q: it must name exactly one of them", found.Remedy, found.PolicyCause, found.Message)
	}
}
