package gitrepo

import (
	"errors"
	"strings"
	"testing"
)

func configSaying(configuration string) Runner {
	return func(string, ...string) ([]byte, error) {
		if configuration == "" {
			return nil, errors.New("exit status 1")
		}
		return []byte(configuration), nil
	}
}

func TestThePromisorRemoteIsNamedByEitherMarkerGitWrites(t *testing.T) {
	cases := []struct {
		name          string
		configuration string
		want          string
		wantOK        bool
	}{
		{"a promisor remote", "remote.origin.promisor true\n", "origin", true},
		{"a remote whose name has a dot", "remote.my.remote.promisor true\n", "my.remote", true},
		{"the extension older git wrote", "extensions.partialclone upstream\n", "upstream", true},
		{"a remote set to false", "remote.origin.promisor false\n", "", false},
		{"no marker", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertPromisorRemote(t, c.configuration, c.want, c.wantOK)
		})
	}
}

func assertPromisorRemote(t *testing.T, configuration, want string, wantOK bool) {
	t.Helper()
	got, ok := PromisorRemote(configSaying(configuration), "repo")

	if got != want || ok != wantOK {
		t.Errorf("PromisorRemote = %q, %v; want %q, %v", got, ok, want, wantOK)
	}
}

func TestTheFetchAdviceListsOnlyTheRevisionsOwnObjectsAndOffersTheSingleObjectFirst(t *testing.T) {
	cases := []struct {
		name          string
		configuration string
		path          string
		want          []string
		avoid         []string
	}{
		{"a path that can be shown", "remote.origin.promisor true\n", "cfg/p.json",
			[]string{"fetch it with git cat-file -e 'abc123:cfg/p.json', or the revision's files with git rev-list --objects --missing=print --no-walk abc123 | sed -n 's/^?//p' | git fetch origin --no-tags --no-write-fetch-head --filter=blob:none --stdin, and run again"}, nil},
		{"no path, as for a whole snapshot", "remote.origin.promisor true\n", "",
			[]string{"fetch the revision's files with git rev-list --objects --missing=print --no-walk abc123"}, []string{"cat-file"}},
		{"a path that cannot be shown", "remote.origin.promisor true\n", "cfg/esc\x1b.json",
			[]string{"fetch the revision's files with"}, []string{"cat-file"}},
		{"a remote whose name a shell would read as a separator", "remote.a;b.promisor true\n", "",
			[]string{"git fetch 'a;b' --no-tags"}, nil},
		{"a repository that is not a partial clone", "", "cfg/p.json", nil, []string{"fetch"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertFetchAdvice(t, c.configuration, c.path, c.want, c.avoid)
		})
	}
}

func assertFetchAdvice(t *testing.T, configuration, path string, want, avoid []string) {
	t.Helper()
	got := FetchAdvice(configSaying(configuration), "repo", "abc123", path)

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

	if got := err.Error(); !strings.Contains(got, "3 objects are not in the local object store") || !strings.HasSuffix(got, "; fetch the revision's files with X and run again") {
		t.Errorf("Error() = %q; want the count, the first path and the advice", got)
	}
}
