package projectconfig

import (
	"errors"
	"strings"
	"testing"
)

// wholeGitOfDiscovery is the whole of the git that WrongCaseAtRevision asks: the revision's tree. Any other question
// is an error that names it, so a test that passes has shown that nothing else was asked.
type wholeGitOfDiscovery struct {
	listingErr error
	asked      []string
}

var discoveredPolicyInTheRevision = fakeRevision{"": {".Lousy-Agents"}, ".Lousy-Agents": {"Coach.json"}}

func (g *wholeGitOfDiscovery) run(_ string, args ...string) ([]byte, error) {
	g.asked = append(g.asked, args[0])
	if args[0] != "ls-tree" {
		return nil, errors.New("unexpected git question: " + strings.Join(args, " "))
	}
	if g.listingErr != nil {
		return nil, g.listingErr
	}
	directory := ""
	for i, arg := range args {
		if arg == "--" {
			directory = strings.TrimSuffix(strings.TrimPrefix(args[i+1], ":(top,literal)"), "/")
		}
	}
	return []byte(strings.Join(discoveredPolicyInTheRevision[directory], "\x00") + "\x00"), nil
}

// A rename the user has made and staged is told what a rename that is not staged is: until the commit, the revision
// still holds the old spelling, and the advice already says to commit a staged one. Nothing but the revision's tree is
// asked, so the answer cannot depend on what the index or the disk holds.
func TestADiscoveredPolicyIsRefusedFromTheRevisionsTreeAlone(t *testing.T) {
	git := &wholeGitOfDiscovery{}

	refusal := WrongCaseAtRevision(git.run, "repo", "HEAD", readPath)

	assertTheRefusalIs(t, refusal, oneSpellingMessage(".Lousy-Agents/Coach.json", readPath))
	for _, question := range git.asked {
		if question != "ls-tree" {
			t.Errorf("git was asked %v; want the revision's tree alone", git.asked)
		}
	}
}

func TestARevisionThatCannotBeListedEndsTheRun(t *testing.T) {
	git := &wholeGitOfDiscovery{listingErr: errors.New("exit status 128: fatal: bad object")}

	refusal := WrongCaseAtRevision(git.run, "repo", "HEAD", readPath)

	if GitSilence(refusal, "coach codesignal", readPath) == nil {
		t.Errorf("WrongCaseAtRevision = %v; want the unanswered listing", refusal)
	}
}

// A policy the user names gets the same refusal, asked before the disk, and the refusal is the working tree's verdict
// as well: the report built from it does not read the disk a second time and say something else. The revision's tree is
// all that is asked, so the answer is the same on any disk.
func TestANamedPolicyIsRefusedFromTheRevisionsTreeBeforeTheDiskIsAsked(t *testing.T) {
	git := &wholeGitOfDiscovery{}

	status, err := ExplicitOnDiskStatus(git.run, "repo", "HEAD", readPath)

	if err != nil || status == nil {
		t.Fatalf("ExplicitOnDiskStatus = %+v, %v; want the refusal", status, err)
	}
	assertTheRefusalIs(t, status, oneSpellingMessage(".Lousy-Agents/Coach.json", readPath))
	if !status.FromWorkingTree {
		t.Errorf("FromWorkingTree is false on %q; want true, so that the report rests on this verdict", status.Message)
	}
	for _, question := range git.asked {
		if question != "ls-tree" {
			t.Errorf("git was asked %v; want the revision's tree alone", git.asked)
		}
	}
}
