package gitrepo

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// A remote's name may hold a dot, and git writes the setting under it as it is. The fake evaluates the pattern
// the code passes, and this reads it from a real configuration, so narrowing the pattern to names without a
// dot fails both.
func TestAPromisorRemoteWhoseNameHoldsADotIsRecognised(t *testing.T) {
	cases := []struct {
		name    string
		remote  string
		setting string
		want    bool
	}{
		{"a dot in the name", "my.remote", "true", true},
		{"two dots in the name", "a.b.c", "true", true},
		{"a dot and a space", "my.remote 2", "true", true},
		{"a dot, set to false", "my.remote", "false", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assertRemoteRecognised(t, c.remote, c.setting, c.want)
		})
	}
}

func assertRemoteRecognised(t *testing.T, remote, setting string, want bool) {
	t.Helper()
	repository := gitfixture.Init(t)
	appendToGitConfig(t, repository, "[remote \""+remote+"\"]\n\tpromisor = "+setting+"\n")

	partial, err := IsPartialClone(RunBytes, repository)
	named, found := PromisorRemote(RunBytes, repository)

	if err != nil || partial != want || found != want || (want && named != remote) {
		t.Errorf("IsPartialClone = %v, %v and PromisorRemote = %q, %v; want %v naming %q", partial, err, named, found, want, remote)
	}
}
