package gitrepo

import (
	"context"
	"os"
	"os/exec"
)

// gitCommand builds every git child of this package: the arguments every read starts with, and the
// caller's environment with two settings added.
//
// GIT_NO_LAZY_FETCH=1 (git 2.44 and later) stops git from fetching an object a partial clone lacks.
// Left on, reading a missing blob runs the remote's transport, which is whatever GIT_SSH_COMMAND,
// core.sshCommand or a credential helper names, and writes the fetched objects into the repository.
// A read the user ran to look at a repository would then execute that program and change the
// repository. An object that is not local is reported as unavailable instead.
//
// GIT_TERMINAL_PROMPT=0 keeps git from asking on the terminal for a credential to a URL that the
// repository's own configuration chose.
//
// Every spawn goes through here, so a runner added later cannot start git without them.
func gitCommand(ctx context.Context, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", gitArgs(dir, args...)...)
	cmd.Env = append(os.Environ(), "GIT_NO_LAZY_FETCH=1", "GIT_TERMINAL_PROMPT=0")
	return cmd
}
