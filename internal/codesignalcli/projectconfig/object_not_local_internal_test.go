package projectconfig

import (
	"errors"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

func runnerOfRepository(partialClone bool) gitrepo.Runner {
	return func(string, ...string) ([]byte, error) {
		if partialClone {
			return []byte("remote.origin.promisor true\n"), nil
		}
		return nil, errors.New("exit status 1")
	}
}

func TestAnObjectThatIsNotLocalIsToldInCoachsWordsAndNotGitsBareExitStatus(t *testing.T) {
	absent := &gitrepo.OperationalError{Message: "m", Cause: &gitrepo.ObjectNotLocalError{Object: "abc"}}
	cases := []struct {
		name         string
		partialClone bool
		path         string
		want         []string
		avoid        []string
	}{
		{"a partial clone and a path that can be shown", true, "cfg/p.json",
			[]string{"could not check whether it is committed at the analyzed revision: its content is not in this partial clone, and Coach does not fetch objects; fetch it with git cat-file -e 'abc123:cfg/p.json' and run again"}, []string{"git said"}},
		{"a partial clone and a name with a control character", true, "cfg/esc\x1b[31m.json",
			[]string{"its content is not in this partial clone"}, []string{"git cat-file", "git said"}},
		{"a partial clone and a path outside the repository", true, "../p.json",
			[]string{"its content is not in this partial clone"}, []string{"git cat-file"}},
		{"a repository that is not a partial clone", false, "cfg/p.json",
			[]string{"its content is not in the local object store, and Coach does not fetch objects"}, []string{"git cat-file", "partial clone", "git said"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertNotLocalWords(t, blobCheckFailure(runnerOfRepository(c.partialClone), "repo", "abc123", c.path, absent).Error(), c.want, c.avoid)
		})
	}
}

func assertNotLocalWords(t *testing.T, got string, want, avoid []string) {
	t.Helper()
	for _, w := range want {
		if !strings.Contains(got, w) {
			t.Errorf("failure %q; want it to contain %q", got, w)
		}
	}
	for _, a := range avoid {
		if strings.Contains(got, a) {
			t.Errorf("failure %q; want it not to contain %q", got, a)
		}
	}
}

func TestAnyOtherFailureOfTheBlobCheckStillQuotesWhatGitSaid(t *testing.T) {
	got := blobCheckFailure(runnerOfRepository(true), "repo", "abc123", "cfg/p.json", errors.New("boom")).Error()

	if !strings.Contains(got, `(git said "boom")`) || strings.Contains(got, "partial clone") {
		t.Errorf("failure %q; want git's own words and no claim about a partial clone", got)
	}
}
