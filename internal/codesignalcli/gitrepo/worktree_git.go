package gitrepo

import (
	"context"
	"os/exec"
)

// worktreeGitCommand builds the git child of a --worktree read. It is gitCommand's child, so it starts with the
// options and the environment every read starts with, among them no optional lock, core.fsmonitor off and no fetch
// of an object a partial clone lacks. A --worktree scan reads the working tree itself, where a repository can
// configure commands git runs on its own, so two things are added.
//
// The hooks directory is emptied by an option on the command line, which outranks the repository's configuration
// and also reaches the git that status starts inside a submodule.
//
// The system-wide configuration is left unread: its settings (a core.excludesFile among them) can hide paths from the
// status that the scan counts, and neither the repository nor the command line chose them. The user's own
// configuration is read, as it is by every other read.
//
// It does not stop every such command: a clean filter can still run when status must compare same-size content, in
// the repository and in each submodule status inspects, so no scan reads file content through git. Changed ranges are
// computed from bytes the scan already holds.
func worktreeGitCommand(ctx context.Context, dir string, args ...string) *exec.Cmd {
	return worktreeChildOf(gitCommand)(ctx, dir, args...)
}

// worktreeChildOf is the builder of a --worktree read made over the builder of every other read. A test gives it a
// builder that runs the real git as one before 2.44 runs it, which does not read the instruction not to fetch.
func worktreeChildOf(build childBuilder) childBuilder {
	return func(ctx context.Context, dir string, args ...string) *exec.Cmd {
		cmd := build(ctx, dir, append([]string{"-c", "core.hooksPath=/dev/null"}, args...)...)
		cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1")
		return cmd
	}
}
