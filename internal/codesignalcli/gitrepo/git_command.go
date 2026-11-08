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
// GIT_LFS_SKIP_SMUDGE=1 tells git-lfs's filter to leave a pointer file as it is and fetch nothing. No read
// here runs a smudge filter, so it is a second line of defence for one that slips in.
//
// The caller's pathspec modes and GIT_CONFIG are removed first (see ambientGitSettings), so the answers do not
// depend on them. The variables that say where the repository is are left as the caller set them.
//
// Every spawn goes through here, so a runner added later cannot start git without them.
func gitCommand(ctx context.Context, dir string, args ...string) *exec.Cmd {
	return commandTold(notToFetch())(ctx, dir, args...)
}

// childBuilder builds the git child of a read. The reads that a test has to run against a git that is not the one
// every read starts (a stalled one, an oversized one, one that does not read the instruction not to fetch) take the
// builder as an argument, and production passes ReadCommandContext, so that no package variable is replaced.
type childBuilder func(ctx context.Context, dir string, args ...string) *exec.Cmd

// notToFetch is the setting that tells git not to fetch an object a partial clone lacks.
func notToFetch() []string { return []string{"GIT_NO_LAZY_FETCH=1"} }

// commandTold is the builder of gitCommand's children with instruction in the place of the one not to fetch. A test
// gives it nothing to run the real git as one before 2.44 runs it, which does not read the setting, so that what such
// a git does with a missing object is observed and not assumed.
func commandTold(instruction []string) childBuilder {
	return func(ctx context.Context, dir string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(ctx, "git", gitArgs(dir, args...)...)
		cmd.Env = append(append(withoutAmbientGitSettings(os.Environ()), instruction...), "GIT_TERMINAL_PROMPT=0", "GIT_LFS_SKIP_SMUDGE=1")
		return cmd
	}
}
