package gitrepo

import (
	"errors"
	"strings"
	"testing"
)

func TestThePromisorRemoteIsNamedByEitherMarkerGitWrites(t *testing.T) {
	cases := []struct {
		name       string
		repository fakeRepository
		want       string
		wantOK     bool
	}{
		{"a promisor remote", fakeRepository{promisors: "remote.origin.promisor true\n"}, "origin", true},
		{"a remote whose name has a dot", fakeRepository{promisors: "remote.my.remote.promisor true\n"}, "my.remote", true},
		{"a remote whose name has a space", fakeRepository{promisors: "remote.my remote.promisor true\n"}, "my remote", true},
		{"the extension older git wrote", fakeRepository{extension: "upstream\n"}, "upstream", true},
		{"a remote set to false", fakeRepository{promisors: "remote.origin.promisor false\n"}, "", false},
		{"no marker", fakeRepository{}, "", false},
		{"a configuration git cannot be asked", fakeRepository{configErr: errors.New("boom")}, "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertPromisorRemote(t, c.repository, c.want, c.wantOK)
		})
	}
}

func assertPromisorRemote(t *testing.T, repository fakeRepository, want string, wantOK bool) {
	t.Helper()
	got, ok := PromisorRemote(repository.run, "repo")

	if got != want || ok != wantOK {
		t.Errorf("PromisorRemote = %q, %v; want %q, %v", got, ok, want, wantOK)
	}
}

func TestTheFetchAdviceListsOnlyTheRevisionsOwnObjectsAndOffersTheSingleObjectFirst(t *testing.T) {
	cases := []struct {
		name       string
		repository fakeRepository
		path       string
		want       []string
		avoid      []string
	}{
		{"a path that can be shown", fakeRepository{promisors: promisorOrigin}, "cfg/p.json",
			[]string{"fetch it with git cat-file -e 'abc123:cfg/p.json', or the revision's files with git rev-list --objects --missing=print --no-walk abc123 | sed -n 's/^?//p' | git fetch origin --no-tags --no-write-fetch-head --filter=blob:none --stdin, and run again"}, nil},
		{"no path, as for a whole snapshot", fakeRepository{promisors: promisorOrigin}, "",
			[]string{"fetch the revision's files with git rev-list --objects --missing=print --no-walk abc123"}, []string{"cat-file"}},
		{"a path that cannot be shown", fakeRepository{promisors: promisorOrigin}, "cfg/esc\x1b.json",
			[]string{"fetch the revision's files with"}, []string{"cat-file"}},
		{"a remote whose name a shell would read as a separator", fakeRepository{promisors: "remote.a;b.promisor true\n"}, "",
			[]string{"git fetch 'a;b' --no-tags"}, nil},
		{"a repository that is not a partial clone", fakeRepository{}, "cfg/p.json", nil, []string{"fetch"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertFetchAdvice(t, c.repository, c.path, c.want, c.avoid)
		})
	}
}

func assertFetchAdvice(t *testing.T, repository fakeRepository, path string, want, avoid []string) {
	t.Helper()
	got := FetchAdvice(repository.run, "repo", "abc123", path)

	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("advice %q; want it to contain %q", got, w)
		}
	}
	for _, a := range avoid {
		if strings.Contains(got, a) {
			t.Errorf("advice %q; want it not to contain %q", got, a)
		}
	}
	if len(want) == 0 && got != "" {
		t.Errorf("advice %q; want none", got)
	}
}

func TestTheMessageForSeveralMissingObjectsCarriesTheAdvice(t *testing.T) {
	err := &ObjectsNotLocalError{Count: 3, FirstPath: "a.go", Advice: "fetch the revision's files with X and run again"}

	if got := err.Error(); !strings.Contains(got, "the revision has 3 objects that are not in the local object store") || !strings.HasSuffix(got, "; fetch the revision's files with X and run again") {
		t.Errorf("Error() = %q; want the count, the first path and the advice", got)
	}
}

func TestObjectsNotLocalErrorSaysOneObjectInTheSingular(t *testing.T) {
	err := &ObjectsNotLocalError{Count: 1, FirstPath: "a.go"}

	got := err.Error()

	if strings.Contains(got, "1 objects") || !strings.Contains(got, "the revision has 1 object that is not in the local object store") {
		t.Errorf("ObjectsNotLocalError = %q, want one object in the singular, worded as the guard words it", got)
	}
}
