package projectconfig

import (
	"testing"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// A submodule at the top level of the repository is the first component of every path inside it, so the probe has
// to try the first component. A policy inside it is told, as one deeper is, that it can be neither restored nor
// committed here, and not to "commit the file", which no commit can do.
func TestAPolicyInsideATopLevelSubmoduleIsTheSubmodulesAndNotUncommitted(t *testing.T) {
	repo := repositoryWithASubmoduleAt(t, "sub")

	for _, path := range []string{"sub/coach.json", "sub/cfg/p.json", "sub"} {
		t.Run(path, func(t *testing.T) { requireTheTopLevelSubmodule(t, repo, path) })
	}
}

func requireTheTopLevelSubmodule(t *testing.T, repo, path string) {
	t.Helper()

	submodule, err := enclosingSubmodule(runProjectConfigGit, repo, path)
	_, cause, unanswered := UncommittedPolicy(runProjectConfigGit, repo, "HEAD", path)

	if err != nil || submodule != "sub" {
		t.Errorf("enclosingSubmodule(%q) = %q, %v; want the submodule \"sub\"", path, submodule, err)
	}
	if unanswered != nil || cause != projectreadiness.CauseInSubmodule {
		t.Errorf("UncommittedPolicy(%q) cause %q, %v; want %q", path, cause, unanswered, projectreadiness.CauseInSubmodule)
	}
}
