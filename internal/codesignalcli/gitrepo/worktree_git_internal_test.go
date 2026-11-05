package gitrepo

import (
	"context"
	"slices"
	"testing"
)

func worktreeGitArgs(args ...string) []string {
	return worktreeGitCommand(context.Background(), "/repo", args...).Args[1:]
}

func TestWorktreeGitCommandSwitchesOffEverythingARepositoryCouldRunOnItsOwn(t *testing.T) {
	assertSwitchedOff(t, worktreeGitArgs("status", "--porcelain=v1", "--untracked-files=all", "-z"))
	assertSwitchedOff(t, worktreeGitArgs("ls-files", "-s", "-z"))
	assertSwitchedOff(t, worktreeGitArgs("config", "--bool", "--get", "core.sparseCheckout"))
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

func TestWorktreeGitCommandLeavesTheArgumentsOfTheReadAsTheyAre(t *testing.T) {
	got := worktreeGitArgs("ls-files", "-s", "-z")

	at := slices.Index(got, "ls-files")
	if at < 0 || !slices.Equal(got[at:], []string{"ls-files", "-s", "-z"}) {
		t.Errorf("git %v, want it to end with the arguments given", got)
	}
}

// A git that can be told not to fetch looks into every submodule, whatever the repository's .gitmodules asks, so that
// a repository cannot hide a dirty submodule, and follows renames.
func TestTheStatusOfAScanOfAGitThatCanBeToldNotToFetchLooksIntoEverySubmoduleAndFollowsRenames(t *testing.T) {
	got := worktreeScanStatusArguments(true, "2.54.0")

	if !slices.Contains(got, "--ignore-submodules=none") {
		t.Errorf("git %v does not inspect every submodule", got)
	}
	if slices.Contains(got, "--no-renames") || slices.Contains(got, "--ignore-submodules=all") {
		t.Errorf("git %v has an argument that the scan of a git that cannot fetch needs and this one does not", got)
	}
}

// A git before 2.44 ignores the instruction not to fetch, so the status it runs is the one every other status gets: it
// does not go into submodules, where a partial clone's transport would run, and does not compare a staged deletion
// with a staged addition, which reads the blob of each.
func TestTheStatusOfAScanOfAGitThatCannotBeToldNotToFetchIsTheStatusOfEveryOtherScan(t *testing.T) {
	assertEveryOtherStatus(t, "2.43.0", true)
	assertEveryOtherStatus(t, "2.17.1", false)
	assertEveryOtherStatus(t, "unknown", false)
}

func assertEveryOtherStatus(t *testing.T, version string, wantNoRenames bool) {
	t.Helper()
	got := worktreeScanStatusArguments(false, version)

	if want := worktreeStatusArguments(false, version); !slices.Equal(got, want) {
		t.Errorf("git %s: worktreeScanStatusArguments = %v, want the arguments of every other status, %v", version, got, want)
	}
	if !slices.Contains(got, "--ignore-submodules=all") {
		t.Errorf("git %s: %v goes into submodules", version, got)
	}
	if slices.Contains(got, "--no-renames") != wantNoRenames {
		t.Errorf("git %s: %v, want --no-renames = %v", version, got, wantNoRenames)
	}
}
