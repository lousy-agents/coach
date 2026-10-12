package revisionfs

import (
	"context"
	"os"
	"os/exec"
	"time"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// Snapshot-read boundary budgets. Unlike projectconfig.MaxBytes
// (sized for one small config document), these bound a whole-tree listing
// and arbitrary tracked source files, so they are deliberately larger while
// still finite: an oversized listing or file fails closed instead of
// exhausting memory or hanging the CLI (issue #210).
const (
	MaxListBytes         = 64 << 20 // 64 MiB: full `git ls-tree -r` path listing
	maxSnapshotFileBytes = 32 << 20 // 32 MiB: single tracked file's content
	maxSnapshotGitStderr = 64 << 10
	snapshotGitTimeout   = 30 * time.Second
)

// snapshotGitCommandContext builds the git child used by New's
// reads. Unlike gitrepo's default bounded-read command, it never inherits the
// parent process's ambient environment (see sanitizedSnapshotGitEnv).
var snapshotGitCommandContext = func(ctx context.Context, dir string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = sanitizedSnapshotGitEnv()
	return cmd
}

// runSnapshotGit is the git seam used by New and its returned
// fs.FS. Tests may replace it to exercise timeout and bound failures without
// hanging, mirroring projectconfig's runProjectConfigGit.
var runSnapshotGit = func(dir string, maxStdout, maxStderr int64, timeout time.Duration, args ...string) ([]byte, error) {
	return gitrepo.RunBytesBoundedWith(snapshotGitCommandContext, dir, maxStdout, maxStderr, timeout, args...)
}

// sanitizedSnapshotGitEnv returns the minimal environment for every git
// child a snapshot read spawns. It never forwards the parent process's
// ambient environment wholesale: only PATH (so the git executable can be
// found) and HOME are carried through, plus GIT_TERMINAL_PROMPT/
// GIT_CONFIG_NOSYSTEM to force non-interactive, system-config-free
// behavior, and GIT_NO_LAZY_FETCH to disable partial-clone promisor fetches
// so a blobless snapshot can never reach the network. Go-tooling and
// proxy/credential variables (GOPROXY, GOFLAGS,
// GOPATH, GO111MODULE, GONOSUMCHECK, GOSUMDB, HTTP(S)_PROXY, NO_PROXY, ...)
// are deliberately never forwarded, even when set in the parent process:
// this package only runs `git ls-tree`/`git show` against a local
// repository, which needs none of them, and forwarding them would be an
// unnecessary vector for those settings to influence a read that must stay
// local and hermetic.
func sanitizedSnapshotGitEnv() []string {
	env := []string{
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_NO_LAZY_FETCH=1",
	}
	if value, ok := os.LookupEnv("PATH"); ok {
		env = append(env, "PATH="+value)
	}
	if value, ok := os.LookupEnv("HOME"); ok {
		env = append(env, "HOME="+value)
	}
	return env
}
