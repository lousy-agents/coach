package gitrepo

import (
	"context"
	"slices"
	"testing"
)

// callerVariables are set by whatever started Coach, and say where git finds the repository, its index and its
// configuration. Every read of the scan takes them as the caller set them, so that the files, the index and the
// revisions one scan reads belong to one repository.
var callerVariables = []string{
	"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_OBJECT_DIRECTORY", "GIT_NAMESPACE",
	"GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM", "GIT_CONFIG_COUNT", "GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0",
	"GIT_CONFIG_PARAMETERS", "XDG_CONFIG_HOME",
}

func TestWorktreeGitCommandRunsInTheEnvironmentOfEveryOtherReadWithoutTheSystemConfiguration(t *testing.T) {
	for _, name := range callerVariables {
		t.Setenv(name, "/set-by-the-caller")
	}

	got := worktreeGitCommand(context.Background(), "/repo", "status").Env

	want := append(gitCommand(context.Background(), "/repo", "status").Env, "GIT_CONFIG_NOSYSTEM=1")
	if !slices.Equal(got, want) {
		t.Errorf("Env = %v, want the environment of gitCommand and GIT_CONFIG_NOSYSTEM=1", got)
	}
	for _, name := range callerVariables {
		if !slices.Contains(got, name+"=/set-by-the-caller") {
			t.Errorf("Env lacks %s as the caller set it", name)
		}
	}
	if !slices.Contains(got, "GIT_NO_LAZY_FETCH=1") {
		t.Errorf("Env = %v, want the instruction not to fetch an object a partial clone lacks", got)
	}
}
