package gitrepo

import (
	"errors"
	"reflect"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

func TestPathExistsAtRevisionMatchesAnEntryOfAnyTypeByItsExactSpelling(t *testing.T) {
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "cfg/p.json", "{}")
	cases := []struct {
		name string
		path string
		want bool
	}{
		{"a file", "cfg/p.json", true},
		{"a directory", "cfg", true},
		{"a file in another case", "cfg/P.JSON", false},
		{"a directory in another case", "CFG", false},
		{"a name nothing holds", "cfg/other.json", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertPathExistsAtRevision(t, repo, c.path, c.want)
		})
	}
}

func assertPathExistsAtRevision(t *testing.T, repo, path string, want bool) {
	t.Helper()
	got, err := PathExistsAtRevision(RunBytes, repo, "HEAD", path)

	if err != nil || got != want {
		t.Errorf("PathExistsAtRevision(%q) = %v, %v; want %v", path, got, err, want)
	}
}

func TestPathExistsAtRevisionAsksForTheLiteralPathFromTheRepositoryRoot(t *testing.T) {
	var gotArgs []string
	run := func(_ string, args ...string) ([]byte, error) {
		gotArgs = args
		return nil, nil
	}

	if _, err := PathExistsAtRevision(run, "repo", "abc123", "a[1]*.json"); err != nil {
		t.Fatalf("PathExistsAtRevision: %v", err)
	}

	want := []string{"ls-tree", "--full-tree", "--name-only", "abc123", "--", ":(top,literal)a[1]*.json"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Errorf("git args = %v; want %v", gotArgs, want)
	}
}

func TestPathExistsAtRevisionReportsGitsFailureAsAnErrorNotAsAbsence(t *testing.T) {
	run := func(string, ...string) ([]byte, error) { return nil, errors.New("boom") }

	got, err := PathExistsAtRevision(run, "repo", "HEAD", "p.json")

	if err == nil || got {
		t.Errorf("PathExistsAtRevision = %v, %v; want an error and no claim that the path exists", got, err)
	}
}
