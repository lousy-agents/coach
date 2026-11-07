package revisionfs

import (
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

// snapshotGitCommandContext builds the git child used by New's reads. It is gitrepo's own, unchanged: the
// child starts with the environment every other read of the repository starts with, so a snapshot read sees
// the repository the way the policy and the changed-file reads do. A setting such as GIT_NO_REPLACE_OBJECTS
// then applies to all of them or to none, and no report describes two versions of one file. A setting every
// read needs belongs in gitrepo's gitCommand, not here.
var snapshotGitCommandContext = gitrepo.ReadCommandContext

// runSnapshotGit is the git seam used by New and its returned
// fs.FS. Tests may replace it to exercise timeout and bound failures without
// hanging, mirroring projectconfig's runProjectConfigGit.
var runSnapshotGit = func(dir string, maxStdout, maxStderr int64, timeout time.Duration, args ...string) ([]byte, error) {
	return gitrepo.RunBytesBoundedWith(snapshotGitCommandContext, dir, maxStdout, maxStderr, timeout, args...)
}
