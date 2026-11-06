package gitrepo

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// leakingVariables redirect where git looks for the repository, its index and
// its configuration, so none of them may reach a status read.
var leakingVariables = []string{
	"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_NAMESPACE",
	"GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0",
	"GIT_CONFIG_PARAMETERS", "XDG_CONFIG_HOME",
}

func TestWorktreeGitCommandRunsInAnEnvironmentThatCannotRedirectGit(t *testing.T) {
	for _, name := range leakingVariables {
		t.Setenv(name, "/redirected")
	}

	cmd := worktreeGitCommand(context.Background(), "/repo", "status")

	if cmd.Env == nil {
		t.Fatal("Env is nil, so the child inherits the whole ambient environment")
	}
	for _, entry := range cmd.Env {
		name, _, _ := strings.Cut(entry, "=")
		if slices.Contains(leakingVariables, name) {
			t.Errorf("Env carries %s, which can redirect git away from the repository being read", entry)
		}
	}
}
