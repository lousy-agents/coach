package gitrepo

import (
	"context"
	"os/exec"
	"testing"
)

// The children a test hands to the code under test: each is the whole of git for the test (a stalled one, an oversized
// one, a failing one), and none replaces anything in the package, so no test runs a git that is partly real and
// partly pretended.

// childWhateverIsAsked builds child for every git command, whatever its arguments.
func childWhateverIsAsked(child func(ctx context.Context) *exec.Cmd) childBuilder {
	return func(ctx context.Context, _ string, _ ...string) *exec.Cmd { return child(ctx) }
}

// streamChild builds child for a `cat-file --batch` stream (the snapshot's, or the baseline reader's with -Z). Any
// other command fails the test, so the fake cannot be asked a question it does not answer.
func streamChild(t *testing.T, child func(ctx context.Context) *exec.Cmd) childBuilder {
	t.Helper()
	return func(ctx context.Context, _ string, args ...string) *exec.Cmd {
		if len(args) < 2 || args[0] != "cat-file" || args[1] != "--batch" {
			t.Errorf("unexpected git command %v: the fake git answers only a cat-file --batch stream", args)
			return exec.CommandContext(ctx, "false")
		}
		return child(ctx)
	}
}

// runnerHeldTo is a Runner that answers each question with the child that answer builds from the git arguments, in
// place of git, and holds every read to replacement.
func runnerHeldTo(replacement budget, answer func(ctx context.Context, args []string) *exec.Cmd) Runner {
	build := func(ctx context.Context, _ string, args ...string) *exec.Cmd { return answer(ctx, args) }
	return func(dir string, args ...string) ([]byte, error) {
		return runBytesBy(build, func([]string) budget { return replacement }, dir, args...)
	}
}

// childRunnerHeldTo is runnerHeldTo for a child that is the same whatever is asked.
func childRunnerHeldTo(replacement budget, child func(ctx context.Context) *exec.Cmd) Runner {
	return runnerHeldTo(replacement, func(ctx context.Context, _ []string) *exec.Cmd { return child(ctx) })
}

// readObjectsFrom reads oids with the git that build starts, and takes no stream for an object past the limit.
func readObjectsFrom(build childBuilder, dir string, oids []string, budget snapshotBudget, visit func(oid string, content []byte) error) (objectOutcome, error) {
	return readObjects(build, dir, oids, budget, visit, nil)
}

// readObjectsOf reads oids with the real git.
func readObjectsOf(dir string, oids []string, budget snapshotBudget, visit func(oid string, content []byte) error) (objectOutcome, error) {
	return readObjects(ReadCommandContext, dir, oids, budget, visit, nil)
}
