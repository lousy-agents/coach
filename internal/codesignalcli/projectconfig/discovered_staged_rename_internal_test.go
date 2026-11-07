package projectconfig

import (
	"errors"
	"strings"
	"testing"
)

const discoveredPath = ".lousy-agents/coach.json"

var discoveredPolicyInTheRevision = fakeRevision{"": {".Lousy-Agents"}, ".Lousy-Agents": {"Coach.json"}}

// wholeGitOfDiscovery is the whole of the git that WrongCaseAtRevision asks: the revision's tree, the index's entries
// for the path, and where the repository's root is. Any other question is an error that names it.
type wholeGitOfDiscovery struct {
	root       string
	index      string
	indexErr   error
	listingErr error
	asked      []string
}

func (g *wholeGitOfDiscovery) run(_ string, args ...string) ([]byte, error) {
	g.asked = append(g.asked, args[0])
	switch args[0] {
	case "ls-tree":
		if g.listingErr != nil {
			return nil, g.listingErr
		}
		directory := ""
		if at := indexOf(args, "--"); at >= 0 {
			directory = strings.TrimSuffix(strings.TrimPrefix(args[at+1], ":(top,literal)"), "/")
		}
		return []byte(strings.Join(discoveredPolicyInTheRevision[directory], "\x00") + "\x00"), nil
	case "ls-files":
		return []byte(g.index), g.indexErr
	case "rev-parse":
		return []byte(g.root + "\n"), nil
	}
	return nil, errors.New("unexpected git question: " + strings.Join(args, " "))
}

func indexOf(args []string, want string) int {
	for i, arg := range args {
		if arg == want {
			return i
		}
	}
	return -1
}

// A discovered policy is held to the same rule as a named one: when the user has moved it with git mv and not yet
// committed, the index records the spelling Coach reads, the move the refusal prints would fail, and the commit is all
// that is left. It was only the named path that knew this.
func TestADiscoveredPolicyWhoseRenameIsStagedIsLeftToTheCommit(t *testing.T) {
	git := &wholeGitOfDiscovery{root: t.TempDir(), index: indexRecord("100644", "0", discoveredPath)}

	refusal := WrongCaseAtRevision(git.run, "repo", "HEAD", discoveredPath)

	if refusal != nil {
		t.Errorf("WrongCaseAtRevision = %v; want nil, which leaves the policy plainly uncommitted", refusal)
	}
}

func TestADiscoveredPolicyWhoseRenameIsNotStagedIsStillRefusedWithTheMove(t *testing.T) {
	git := &wholeGitOfDiscovery{root: t.TempDir()}

	refusal := WrongCaseAtRevision(git.run, "repo", "HEAD", discoveredPath)

	if refusal == nil || !strings.Contains(refusal.Error(), "git -C \"$top\" mv -- '.Lousy-Agents/Coach.json' '.lousy-agents/coach.json'") {
		t.Errorf("WrongCaseAtRevision = %v; want the refusal with the move", refusal)
	}
}

// An index that cannot be asked is a question git left unanswered, which ends the run as the listing's failure does,
// and neither the move nor the commit is advised on a guess.
func TestAnIndexThatCannotBeAskedAboutADiscoveredPolicyEndsTheRun(t *testing.T) {
	git := &wholeGitOfDiscovery{root: t.TempDir(), indexErr: errors.New("exit status 128: fatal: unable to read the index")}

	refusal := WrongCaseAtRevision(git.run, "repo", "HEAD", discoveredPath)

	if GitSilence(refusal, "coach codesignal", discoveredPath) == nil {
		t.Errorf("WrongCaseAtRevision = %v; want a question git left unanswered", refusal)
	}
}

func TestARevisionThatCannotBeListedIsPassedOnWithoutAskingTheIndex(t *testing.T) {
	git := &wholeGitOfDiscovery{root: t.TempDir(), listingErr: errors.New("exit status 128: fatal: bad object"), index: indexRecord("100644", "0", discoveredPath)}

	refusal := WrongCaseAtRevision(git.run, "repo", "HEAD", discoveredPath)

	if GitSilence(refusal, "coach codesignal", discoveredPath) == nil {
		t.Errorf("WrongCaseAtRevision = %v; want the unanswered listing", refusal)
	}
	for _, question := range git.asked {
		if question == "ls-files" {
			t.Errorf("the index was asked about although the listing was not answered: %v", git.asked)
		}
	}
}
