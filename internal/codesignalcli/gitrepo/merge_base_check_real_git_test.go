package gitrepo

import (
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// The merge base check for a git that can be told not to fetch: a baseline asks nothing, since the snapshot
// reports HEAD's own objects, and a diff is refused for the merge base alone, with no blame on the version.
func TestAGitThatCanBeToldNotToFetchIsNeverRefusedForItsVersion(t *testing.T) {
	clone := filteredClone(t, twoCommitSource(t), false)

	baseline := refuseWhenTheMergeBaseLacksObjects(RunBytes, clone, "")
	diff := refuseWhenTheMergeBaseLacksObjects(RunBytes, clone, "HEAD~1")

	if baseline != nil {
		t.Errorf("baseline: RefuseMissingObjects = %v; want nothing, the snapshot reports HEAD's objects", baseline)
	}
	if text := refusalText(diff); !strings.Contains(text, "the merge base of HEAD~1 and HEAD") || strings.Contains(text, "cannot be told") {
		t.Errorf("diff: RefuseMissingObjects = %v; want the merge base's objects named and no blame on the version", diff)
	}
}

func TestARepositoryThatIsNotAPartialCloneIsNeverTakenForOneWhateverItsRemotesSayAboutPromising(t *testing.T) {
	settings := map[string]string{
		"no setting":                       "",
		"a promisor setting of false":      "\tpromisor = false\n",
		"a filter and no promisor":         "\tpartialclonefilter = blob:none\n",
		"an empty extensions.partialclone": "",
	}
	for name, setting := range settings {
		t.Run(name, func(t *testing.T) {
			assertNotTakenForAPartialClone(t, setting, name == "an empty extensions.partialclone")
		})
	}
}

func assertNotTakenForAPartialClone(t *testing.T, setting string, emptyExtension bool) {
	t.Helper()
	repository := gitfixture.Init(t)
	gitfixture.CommitFile(t, repository, "a.go", "package a\n")
	appendToGitConfig(t, repository, "[remote \"origin\"]\n\turl = https://example.invalid/r.git\n"+setting)
	if emptyExtension {
		gitOutput(t, repository, "config", "extensions.partialclone", "")
	}

	partial, err := IsPartialClone(RunBytes, repository)

	if err != nil || partial {
		t.Errorf("IsPartialClone = %v, %v; want an ordinary repository, so nothing is refused for it", partial, err)
	}
}
