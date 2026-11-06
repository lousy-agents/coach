package gitrepo

import (
	"context"
	"reflect"
	"slices"
	"testing"
)

func worktreeGitArgs(args ...string) []string {
	return worktreeGitCommand(context.Background(), "/repo", args...).Args[1:]
}

func TestWorktreeGitCommandSwitchesOffEverythingARepositoryCouldRunOnItsOwn(t *testing.T) {
	for _, args := range [][]string{
		{"status", "--porcelain=v1", "--untracked-files=all", "-z"},
		{"ls-files", "-s", "-z"},
		{"config", "--bool", "--get", "core.sparseCheckout"},
	} {
		t.Run(args[0], func(t *testing.T) {
			assertSwitchedOff(t, worktreeGitArgs(args...))
		})
	}
}

// assertSwitchedOff checks the options that outrank repository configuration:
// a hook directory with nothing in it, no fsmonitor, and no optional lock.
func assertSwitchedOff(t *testing.T, got []string) {
	t.Helper()
	for _, option := range []string{"core.hooksPath=/dev/null", "core.fsmonitor=false"} {
		if !hasConfigOption(got, option) {
			t.Errorf("git %v lacks -c %s, which a repository's own configuration would override", got, option)
		}
	}
	if !slices.Contains(got, "--no-optional-locks") {
		t.Errorf("git %v can take an optional lock, so a status refresh may rewrite the index and run post-index-change", got)
	}
}

// hasConfigOption reports whether args pass setting to git as `-c setting`.
func hasConfigOption(args []string, setting string) bool {
	at := slices.Index(args, setting)
	return at > 0 && args[at-1] == "-c"
}

func TestWorktreeGitCommandMakesStatusReportASubmoduleTheRepositoryAsksToIgnore(t *testing.T) {
	got := worktreeGitArgs("status", "--porcelain=v1", "-z")

	at := slices.Index(got, "status")
	want := []string{"status", "--ignore-submodules=none", "--porcelain=v1", "-z"}
	if at < 0 || !reflect.DeepEqual(got[at:], want) {
		t.Errorf("git %v, want it to end with %v", got, want)
	}
}

func TestWorktreeGitCommandPassesNoSubmoduleOptionToCommandsThatDoNotTakeOne(t *testing.T) {
	for _, args := range [][]string{{"ls-files", "-s", "-z"}, {"config", "--get", "core.ignorecase"}} {
		t.Run(args[0], func(t *testing.T) {
			assertNoSubmoduleOption(t, args)
		})
	}
}

func assertNoSubmoduleOption(t *testing.T, args []string) {
	t.Helper()
	if got := worktreeGitArgs(args...); slices.Contains(got, "--ignore-submodules=none") {
		t.Errorf("git %v: %s does not accept --ignore-submodules", got, args[0])
	}
}
