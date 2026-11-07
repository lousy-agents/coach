package gitrepo

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func presentIn(env []string, key string) bool {
	return slices.ContainsFunc(env, func(entry string) bool { return strings.HasPrefix(entry, key+"=") })
}

// Each of these, set by whatever started Coach, changes what git reads or which repository it reads. Coach
// asks git about one repository, `-C dir`, with literal pathspecs, so its answers must not depend on them.
// The list is spelled out here and not taken from the code, so a variable removed from the code fails this.
var settingsThatChangeWhatGitReads = []string{
	// how every pathspec is read, including the literal ones Coach builds
	"GIT_LITERAL_PATHSPECS", "GIT_GLOB_PATHSPECS", "GIT_NOGLOB_PATHSPECS", "GIT_ICASE_PATHSPECS",
	// which repository, work tree and index git reads, and how it finds them
	"GIT_DIR", "GIT_WORK_TREE", "GIT_COMMON_DIR", "GIT_INDEX_FILE", "GIT_NAMESPACE",
	"GIT_CEILING_DIRECTORIES", "GIT_DISCOVERY_ACROSS_FILESYSTEM",
	// which objects git reads
	"GIT_OBJECT_DIRECTORY", "GIT_ALTERNATE_OBJECT_DIRECTORIES",
}

func TestGitCommandDropsEverySettingOfTheCallerThatChangesWhatGitReads(t *testing.T) {
	for _, key := range settingsThatChangeWhatGitReads {
		t.Run(key, func(t *testing.T) {
			t.Setenv(key, "1")

			env := gitCommand(context.Background(), "repo", "status").Env

			if presentIn(env, key) {
				t.Errorf("%s reaches git from the caller's environment: it changes what git reads, and Coach's answers would depend on it", key)
			}
		})
	}
}

// What the caller needs for git to run at all, and to find its configuration, is left as it is.
func TestGitCommandLeavesTheCallerWhatGitNeedsToRun(t *testing.T) {
	kept := map[string]string{
		"PATH": "/somewhere/bin:/usr/bin", "HOME": "/home/someone", "GIT_CONFIG_GLOBAL": "/home/someone/.gitconfig",
		"GIT_SSH_COMMAND": "ssh -F /home/someone/ssh_config", "LANG": "C", "TMPDIR": "/var/tmp",
	}
	for key, value := range kept {
		t.Setenv(key, value)
	}

	env := gitCommand(context.Background(), "repo", "status").Env

	for key, value := range kept {
		if got := effectiveValue(env, key); got != value {
			t.Errorf("%s = %q, want %q: git needs it as the caller set it", key, got, value)
		}
	}
}
