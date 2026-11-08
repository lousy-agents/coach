package gitrepo

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Each read that ResolveRevisions and ResolveBaselineRevision make reports its own timeout, and none of them
// is taken for the answer "no" that the same read gives when git does answer.

const resolveBudgetTimeout = 300 * time.Millisecond

// stalledGit is a git that answers the questions about a worktree and stalls on the one whose arguments
// start with stalled, so that each read can be shown to report its own timeout.
type stalledGit struct{ stalled string }

func (g stalledGit) answer(ctx context.Context, args []string) *exec.Cmd {
	joined := strings.Join(args, " ")
	if strings.HasPrefix(joined, g.stalled) {
		return exec.CommandContext(ctx, "sleep", "30")
	}
	if strings.Contains(joined, "--is-inside-work-tree") {
		return exec.CommandContext(ctx, "sh", "-c", "printf true")
	}
	return exec.CommandContext(ctx, "sh", "-c", "printf 0123456789abcdef0123456789abcdef01234567")
}

// maskedAnswers are the messages a read that answered no gives; a read that did not answer gives none of them.
var maskedAnswers = []string{"is not inside a Git worktree", "is not readable", "cannot be resolved", "no merge base"}

func requireNoMaskedAnswer(t *testing.T, err error) {
	t.Helper()
	for _, masked := range maskedAnswers {
		if strings.Contains(err.Error(), masked) {
			t.Errorf("error = %q: a git that did not answer was reported as one that said %q", err, masked)
		}
	}
}

func requireTimedOutRead(t *testing.T, err error, want string) {
	t.Helper()
	var operational *OperationalError
	if !errors.As(err, &operational) || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want an operational error containing %q", err, want)
	}
	requireNoMaskedAnswer(t, err)
}

type stalledResolution struct{ stalled, want string }

func (c stalledResolution) run(t *testing.T) {
	run := runnerHeldTo(budget{timeout: resolveBudgetTimeout, stdout: 1 << 20, stderr: 1 << 16}, stalledGit{c.stalled}.answer)

	_, _, err := resolveRevisionsBy(run, ".", "main")

	requireTimedOutRead(t, err, c.want)
}

func TestResolveRevisionsReportsTheReadThatDidNotAnswer(t *testing.T) {
	rev := "git rev-parse: git execution timed out"
	t.Run("rev-parse --is-inside-work-tree", stalledResolution{"rev-parse --is-inside-work-tree", rev}.run)
	t.Run("rev-parse HEAD", stalledResolution{"rev-parse HEAD", rev}.run)
	t.Run("rev-parse --verify", stalledResolution{"rev-parse --verify", rev}.run)
	t.Run("merge-base", stalledResolution{"merge-base", "git merge-base: git execution timed out"}.run)
}

func TestResolveBaselineRevisionReportsTheReadThatDidNotAnswer(t *testing.T) {
	run := runnerHeldTo(budget{timeout: resolveBudgetTimeout, stdout: 1 << 20, stderr: 1 << 16}, stalledGit{"rev-parse"}.answer)

	_, err := resolveHEAD(run, ".")

	requireTimedOutRead(t, err, "git rev-parse: git execution timed out")
}
