package gitrepo

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// The old-git branch of the guard is called here directly, with real git and a real partial clone, and the
// version its message names is an argument: nothing about git is replaced, so these tests prove the decision
// (which revisions are probed, what is refused, in which words) and the probe's reading of real git output.
// They do not prove what an old git would do with a missing object. That the probe itself fetches nothing is
// shown by TestTheProbeFetchesNothingEvenWhenGitIsNotToldNotTo, which runs it without the instruction not to
// fetch, and by a run on a real git 2.39.5 recorded in the checklist. The dispatch on the version, and on
// whether the repository is a partial clone at all, is tested with fakeRepository.
const olderThanTheInstruction = "2.43.0"

// filteredClone is a clone of source made with --filter=blob:none, whose remote is a program that could not
// answer, with or without a checkout. A checkout fetches every blob of HEAD, so HEAD lacks nothing and the
// blobs of earlier commits that differ from it are not local; without one, nothing is local.
func filteredClone(t *testing.T, source string, checkout bool) string {
	t.Helper()
	bare := t.TempDir() + "/bare.git"
	gitOutput(t, source, "clone", "--bare", "--quiet", source, bare)
	gitOutput(t, bare, "config", "uploadpack.allowFilter", "true")
	clone := t.TempDir() + "/part"
	arguments := []string{"clone", "--quiet", "--filter=blob:none"}
	if !checkout {
		arguments = append(arguments, "--no-checkout")
	}
	gitOutput(t, t.TempDir(), append(arguments, "file://"+bare, clone)...)
	gitOutput(t, clone, "remote", "set-url", "origin", "ssh://example.invalid/repo.git")
	return clone
}

func twoCommitSource(t *testing.T) string {
	t.Helper()
	source := gitfixture.Init(t)
	gitfixture.CommitFile(t, source, "a.go", "package a\n")
	gitfixture.CommitFile(t, source, "cfg/p.json", "{}")
	gitfixture.CommitFile(t, source, "a.go", "package a\n\nfunc F() {}\n")
	return source
}

func refusalText(err error) string {
	var refusal *OperationalError
	if !errors.As(err, &refusal) {
		return ""
	}
	return refusal.Message
}

func TestAnOlderGitReadsAPartialCloneWhoseObjectsAreAllHere(t *testing.T) {
	clone := filteredClone(t, twoCommitSource(t), true)

	if err := refuseWhenReadRevisionsLackObjects(RunBytes, clone, "", olderThanTheInstruction); err != nil {
		t.Errorf("RefuseMissingObjects = %v; want the clone read: HEAD lacks nothing", err)
	}
}

func TestAnOlderGitRefusesAPartialCloneThatLacksObjectsOfHead(t *testing.T) {
	clone := filteredClone(t, twoCommitSource(t), false)

	err := refuseWhenReadRevisionsLackObjects(RunBytes, clone, "", olderThanTheInstruction)

	for _, want := range []string{"git 2.43.0 cannot be told not to fetch missing objects", "HEAD has 2 objects that are not in it", "git 2.44 or later", "git rev-list --objects --missing=print --no-walk HEAD"} {
		if !strings.Contains(refusalText(err), want) {
			t.Errorf("RefuseMissingObjects = %v; want it to say %q", err, want)
		}
	}
}

func TestAnOlderGitRefusesADiffWhoseMergeBaseLacksTheObjectsHeadChanged(t *testing.T) {
	clone := filteredClone(t, twoCommitSource(t), true)

	diff := refuseWhenReadRevisionsLackObjects(RunBytes, clone, "HEAD~1", olderThanTheInstruction)
	baseline := refuseWhenReadRevisionsLackObjects(RunBytes, clone, "", olderThanTheInstruction)

	if !strings.Contains(refusalText(diff), "the merge base of HEAD~1 and HEAD has 1 object that is not in it") {
		t.Errorf("diff: RefuseMissingObjects = %v; want the merge base named with the one old version of a.go", diff)
	}
	if baseline != nil {
		t.Errorf("baseline: RefuseMissingObjects = %v; want HEAD alone read", baseline)
	}
}

func TestAnOlderGitRefusesAPartialCloneWhosePromisorSettingHasNoValue(t *testing.T) {
	clone := filteredClone(t, twoCommitSource(t), false)
	config, err := os.ReadFile(clone + "/.git/config")
	if err != nil {
		t.Fatal(err)
	}
	valueless := strings.Replace(string(config), "\tpromisor = true\n", "\tpromisor\n", 1)
	if valueless == string(config) {
		t.Fatal("the clone's configuration did not hold the setting this test rewrites")
	}
	if err := os.WriteFile(clone+"/.git/config", []byte(valueless), 0o644); err != nil {
		t.Fatal(err)
	}

	err = refuseWhenReadRevisionsLackObjects(RunBytes, clone, "", olderThanTheInstruction)

	if !strings.Contains(refusalText(err), "HEAD has 2 objects that are not in it") {
		t.Errorf("RefuseMissingObjects = %v; want the clone recognised as a partial clone and refused", err)
	}
}

// A remote's name may hold a dot, and the setting git wrote for it is the only marker a current git leaves.
func TestAnOlderGitRefusesAPartialCloneWhosePromisorRemoteHasADotInItsName(t *testing.T) {
	clone := filteredClone(t, twoCommitSource(t), false)
	gitOutput(t, clone, "remote", "rename", "origin", "my.remote")

	err := refuseWhenReadRevisionsLackObjects(RunBytes, clone, "", olderThanTheInstruction)

	if !strings.Contains(refusalText(err), "HEAD has 2 objects that are not in it") || !strings.Contains(refusalText(err), "git fetch my.remote") {
		t.Errorf("RefuseMissingObjects = %v; want the dotted remote recognised, refused and named in the fetch", err)
	}
}

func TestAnOlderGitRefusesWhenItCannotTellWhichObjectsAreMissing(t *testing.T) {
	clone := filteredClone(t, twoCommitSource(t), true)

	err := refuseWhenReadRevisionsLackObjects(RunBytes, clone, "no-such-ref", olderThanTheInstruction)

	if !strings.Contains(refusalText(err), "could not tell whether this partial clone has the objects Coach reads") {
		t.Errorf("RefuseMissingObjects = %v; want a refusal for want of an answer, not a read on a guess", err)
	}
}

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
			repository := gitfixture.Init(t)
			gitfixture.CommitFile(t, repository, "a.go", "package a\n")
			appendToGitConfig(t, repository, "[remote \"origin\"]\n\turl = https://example.invalid/r.git\n"+setting)
			if name == "an empty extensions.partialclone" {
				gitOutput(t, repository, "config", "extensions.partialclone", "")
			}

			partial, err := IsPartialClone(RunBytes, repository)

			if err != nil || partial {
				t.Errorf("IsPartialClone = %v, %v; want an ordinary repository, so nothing is refused for it", partial, err)
			}
		})
	}
}
