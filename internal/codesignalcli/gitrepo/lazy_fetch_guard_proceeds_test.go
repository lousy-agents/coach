package gitrepo

import (
	"reflect"
	"testing"
)

func TestRefuseMissingObjectsLeavesAReadAloneWhenNothingCanBeFetched(t *testing.T) {
	cases := []struct {
		name       string
		repository fakeRepository
	}{
		{"a partial clone and a git that can be told not to fetch", fakeRepository{version: currentGit, promisors: promisorOrigin}},
		{"an ordinary clone and an old git", fakeRepository{version: oldGit}},
		{"a partial clone and an old git, every object of HEAD here", fakeRepository{version: oldGit, promisors: promisorOrigin}},
		{"a partial clone named by the extension older git wrote, every object here", fakeRepository{version: oldGit, extension: "origin\n"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertNotRefused(t, c.repository)
		})
	}
}

func assertNotRefused(t *testing.T, repository fakeRepository) {
	t.Helper()

	if err := RefuseMissingObjects(repository.run, "repo", ""); err != nil {
		t.Errorf("RefuseMissingObjects = %v; want a read that fetches nothing to go ahead", err)
	}
}

func TestRefuseMissingObjectsAsksNoQuestionOfGitWhenTheAnswerIsAlreadyKnown(t *testing.T) {
	cases := []struct {
		name       string
		repository fakeRepository
	}{
		{"a repository that is not a partial clone, on a git that cannot fetch", fakeRepository{version: currentGit}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertNothingProbed(t, c.repository)
		})
	}
}

func assertNothingProbed(t *testing.T, repository fakeRepository) {
	t.Helper()

	_ = RefuseMissingObjects(repository.run, "repo", "")

	if len(repository.probed) != 0 {
		t.Errorf("probed %v; a read that cannot fetch has no objects to look for", repository.probed)
	}
}

// The probe is what makes the refusal depend on the objects a read needs: it lists the objects of each
// revision's own tree, and prints only the ones that are not local. --no-walk keeps it to that tree
// instead of every object in history, and --quiet keeps it to the missing ones.
func TestRefuseMissingObjectsProbesTheTreesTheScanReads(t *testing.T) {
	cases := []struct {
		name string
		base string
		want []string
	}{
		{"a baseline reads HEAD", "", []string{"rev-list --objects --missing=print --no-walk --quiet HEAD"}},
		{"a diff asks whether the base's commit is here, then reads the merge base and HEAD", "origin/main", []string{
			"rev-list --missing=print --no-walk --quiet " + firstMissing,
			"rev-list --objects --missing=print --no-walk --quiet " + mergeBaseSHA,
			"rev-list --objects --missing=print --no-walk --quiet HEAD",
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertProbed(t, c.base, c.want)
		})
	}
}

func assertProbed(t *testing.T, base string, want []string) {
	t.Helper()
	repository := fakeRepository{version: oldGit, promisors: promisorOrigin, mergeBase: mergeBaseSHA}

	if err := RefuseMissingObjects(repository.run, "repo", base); err != nil {
		t.Fatalf("RefuseMissingObjects = %v; want every object here to be read", err)
	}

	if !reflect.DeepEqual(repository.probed, want) {
		t.Errorf("probed %q; want %q", repository.probed, want)
	}
}
