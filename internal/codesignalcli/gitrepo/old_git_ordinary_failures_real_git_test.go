package gitrepo

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func aRepositoryWhoseBaseIsMisspelled(t *testing.T) (string, string) {
	t.Helper()
	repository := gitfixture.Init(t)
	gitfixture.CommitFile(t, repository, "a.go", "package a\n")
	return repository, "nosuchref"
}

func aRepositoryWhoseBaseHasNoMergeBase(t *testing.T) (string, string) {
	t.Helper()
	repository := gitfixture.Init(t)
	gitfixture.CommitFile(t, repository, "a.go", "package a\n")
	trunk := strings.TrimSpace(gitfixture.Git(t, repository, "branch", "--show-current"))
	gitfixture.Git(t, repository, "checkout", "--quiet", "--orphan", "other")
	gitfixture.Git(t, repository, "rm", "--quiet", "-rf", ".")
	gitfixture.CommitFile(t, repository, "b.go", "package b\n")
	gitfixture.Git(t, repository, "checkout", "--quiet", trunk)
	return repository, "other"
}

func aDirectoryThatIsNoRepository(t *testing.T) (string, string) {
	t.Helper()
	return t.TempDir(), ""
}

func aRepositoryWithNoCommits(t *testing.T) (string, string) {
	t.Helper()
	return gitfixture.Init(t), ""
}

// A failure whose cause a newer git does not touch is the scan's to word, on a git before 2.44 as on a git that can
// be told not to fetch: the guard of either asks nothing that fetches and refuses nothing. The git is the real one,
// run as an old one runs it.
func TestOnAnOldGitAnOrdinaryFailureIsLeftToTheScanAsOnACurrentGit(t *testing.T) {
	asGitBefore244(t)
	assertLeftToTheScanOnBothGits(t, "a misspelled --base", aRepositoryWhoseBaseIsMisspelled)
	assertLeftToTheScanOnBothGits(t, "a --base with no merge base", aRepositoryWhoseBaseHasNoMergeBase)
	assertLeftToTheScanOnBothGits(t, "a directory that is no repository", aDirectoryThatIsNoRepository)
	assertLeftToTheScanOnBothGits(t, "a repository with no commits", aRepositoryWithNoCommits)
}

func assertLeftToTheScanOnBothGits(t *testing.T, name string, arrange func(*testing.T) (string, string)) {
	t.Helper()
	t.Run(name, func(t *testing.T) {
		directory, base := arrange(t)

		old := refuseWhenReadRevisionsLackObjects(RunBytes, directory, base, olderThanTheInstruction)
		current := RefuseMissingObjects(RunBytes, directory, base)

		if old != nil || current != nil {
			t.Errorf("the guard of a git before 2.44 said %v, and of a current git %v; want neither to refuse what the scan words itself", old, current)
		}
	})
}
