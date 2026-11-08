package projectconfig

import "testing"

// core.sparseCheckout is a git boolean. A person can set it as yes, on or 1, `git sparse-checkout
// disable` leaves it as false, and git answers all of those as true or false only when it is asked
// for a boolean. These tests run real git, because what git prints for each spelling is the thing
// being relied on: a canned answer cannot tell a command that asks for a boolean from one that does
// not, and cannot answer false at all.

const sparsePolicy = "pol.json"

// hideBySparseCheckout leaves the committed policy out of a real sparse checkout, so git marks its
// index entry skip-worktree itself, and then writes core.sparseCheckout in the given spelling. git
// keeps the setting of a sparse checkout in the worktree config, which wins over the shared one, so
// the spelling has to go there to be the one git reads.
func hideBySparseCheckout(t *testing.T, repo, spelling string) {
	t.Helper()
	commitPolicy(t, repo, sparsePolicy, validPolicy, 0o644)
	runGit(t, repo, "sparse-checkout", "set", "--no-cone", "/a.go")
	runGit(t, repo, "config", "--worktree", "core.sparseCheckout", spelling)
}

// markByHandWhileCoreSparseCheckoutIs marks the policy's entry skip-worktree and leaves
// core.sparseCheckout in the given spelling.
func markByHandWhileCoreSparseCheckoutIs(t *testing.T, repo, spelling string) {
	t.Helper()
	commitPolicy(t, repo, sparsePolicy, validPolicy, 0o644)
	runGit(t, repo, "update-index", "--skip-worktree", sparsePolicy)
	runGit(t, repo, "config", "core.sparseCheckout", spelling)
}

// assertSpelling judges the policy in a repository arranged for one spelling of core.sparseCheckout.
func assertSpelling(t *testing.T, arrange func(t *testing.T, repo, spelling string), spelling string, want restoreVerdict) {
	t.Helper()
	assertJudgeRestore(t, sparsePolicy, func(t *testing.T, repo string) { arrange(t, repo, spelling) }, want)
}

func TestAPolicyLeftOutOfASparseCheckoutIsNamedWhicheverWayGitSpellsTrue(t *testing.T) {
	for _, spelling := range []string{"true", "yes", "on", "1"} {
		t.Run("core.sparseCheckout "+spelling, func(t *testing.T) {
			assertSpelling(t, hideBySparseCheckout, spelling, restoreSkippedBySparseCheckout)
		})
	}
}

func TestASkipWorktreeMarkSetByHandIsNotASparseCheckoutWhicheverWayGitSpellsFalse(t *testing.T) {
	for _, spelling := range []string{"false", "no", "off", "0"} {
		t.Run("core.sparseCheckout "+spelling, func(t *testing.T) {
			assertSpelling(t, markByHandWhileCoreSparseCheckoutIs, spelling, restoreSkipWorktreeMarked)
		})
	}
}

func TestASkipWorktreeMarkSetByHandAfterASparseCheckoutWasDisabledIsNotASparseCheckout(t *testing.T) {
	assertJudgeRestore(t, sparsePolicy, func(t *testing.T, repo string) {
		commitPolicy(t, repo, sparsePolicy, validPolicy, 0o644)
		runGit(t, repo, "sparse-checkout", "set", "--no-cone", "/a.go")
		runGit(t, repo, "sparse-checkout", "disable")
		runGit(t, repo, "update-index", "--skip-worktree", sparsePolicy)
	}, restoreSkipWorktreeMarked)
}

func TestASkipWorktreeMarkIsNotASparseCheckoutWhenCoreSparseCheckoutIsUnset(t *testing.T) {
	assertJudgeRestore(t, sparsePolicy, func(t *testing.T, repo string) {
		commitPolicy(t, repo, sparsePolicy, validPolicy, 0o644)
		runGit(t, repo, "update-index", "--skip-worktree", sparsePolicy)
	}, restoreSkipWorktreeMarked)
}

func TestACoreSparseCheckoutGitRefusesAsABooleanLeavesTheVerdictUndecided(t *testing.T) {
	assertSpelling(t, markByHandWhileCoreSparseCheckoutIs, "maybe", restoreUndecidable)
}
