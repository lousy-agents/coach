package gitrepo

import (
	"context"
	"os/exec"
)

// worktreeGitCommand builds the git child for the working-tree status reads. A
// repository can configure commands git runs on its own while reading the
// working tree, so the child gets the sanitized environment, and the options
// that outrank repository configuration are on the command line: fsmonitor and
// hooks are switched off (a command-line -c also reaches the git that status
// starts inside a submodule), and status takes no optional lock, so it never
// rewrites the index and never reaches the post-index-change hook.
//
// It does not stop every such command: a clean filter can still run when status
// must compare same-size content, in the repository and in each submodule
// status inspects, so no scan reads file content through git. Changed ranges
// are computed from bytes the scan already holds.
//
// Status inspects every submodule, whatever the repository's .gitmodules says:
// its ignore setting would otherwise let a hostile repository hide a dirty
// submodule from the scan.
func worktreeGitCommand(ctx context.Context, dir string, args ...string) *exec.Cmd {
	command := append([]string{
		"--no-optional-locks",
		"-C", dir,
		"-c", "core.fsmonitor=false",
		"-c", "core.hooksPath=/dev/null",
	}, inspectingSubmodules(args)...)
	cmd := exec.CommandContext(ctx, "git", command...)
	cmd.Env = SanitizedGitEnv()
	return cmd
}

// inspectingSubmodules adds the option that makes `git status` report a dirty
// submodule whatever the repository configures. Every other subcommand is left
// as it is, because it does not accept the option.
func inspectingSubmodules(args []string) []string {
	if len(args) == 0 || args[0] != "status" {
		return args
	}
	return append([]string{"status", "--ignore-submodules=none"}, args[1:]...)
}
