package projectconfig

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
)

// The note about git add --sparse is advice for a repository whose sparse checkout is on: core.sparseCheckout
// set to false, or never set, is a repository whose directories are all on disk.
func TestTheSparseNoteIsOfferedOnlyWhereCoreSparseCheckoutIsOn(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  bool
	}{
		{"on", "true", true},
		{"set to false", "false", false},
		{"never set", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { requireTheSparseNote(t, c.value, c.want) })
	}
}

func requireTheSparseNote(t *testing.T, value string, want bool) {
	t.Helper()
	repo := gitfixture.Init(t)
	gitfixture.CommitFile(t, repo, "a.txt", "x")
	if value != "" {
		gitfixture.Git(t, repo, "config", "core.sparseCheckout", value)
	}

	if got := newPolicyMayNeedSparse(gitrepo.RunBytes, repo, "cfg/p.json"); got != want {
		t.Errorf("newPolicyMayNeedSparse with core.sparseCheckout %q = %v; want %v", value, got, want)
	}
}
