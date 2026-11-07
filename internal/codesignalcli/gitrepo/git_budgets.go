package gitrepo

import (
	"slices"
	"time"
)

// budget is what one of the scan's own git reads may use: the wall time git has to answer, and the
// output Coach will hold. A read that is not given one waits for as long as git does, and git waits as
// long as a file system, a pipe or a helper makes it, so no read on a scan's path is without one.
type budget struct {
	timeout time.Duration
	stdout  int64
	stderr  int64
}

// The numbers follow the bounds the policy and snapshot reads already run under (revisionfs: 30 s, a
// 64 MiB listing, a 32 MiB file, 64 KiB of stderr).
const (
	gitStderrBudget = 64 << 10
	// A question git answers from a few files under .git: whether this is a worktree, what HEAD is, whether
	// two commits share a base, how the repository is configured, which version git is. Any of them taking
	// 30 s means git is stuck, not busy.
	metadataTimeout = 30 * time.Second
	metadataBytes   = 1 << 20
	// A full listing of the tree, or one file's content: the same limits revisionfs holds its listing and
	// its file reads to.
	listingBytes = 64 << 20
	fileBytes    = 32 << 20
	// The changed files of a diff with rename and copy detection (--find-copies-harder) is CPU work that
	// grows with the repository, so it gets longer than a lookup. It prints one record per changed path.
	// The diff of one path is a lookup like git show: it reads two blobs.
	diffListingTimeout = 2 * time.Minute
	// A git subcommand no one has named: a small read.
	unnamedBytes = 16 << 20
)

// The snapshot of a revision is read as one stream (`git cat-file --batch`) that is started and fed by its
// own code, so it does not go through RunBytes and has no row in defaultBudgetFor; these are its row.
//
//   - SnapshotWallTime holds the whole stream. 10 minutes is long for any repository this fits, and a git that
//     stops answering does not hold the scan past it.
//   - SnapshotObjectBytes holds one object, which is what the scan keeps in memory: each object is written to
//     disk before the next is read. 512 MiB is the working-set budget the report states for an analysis, so
//     an object larger than that is larger than the scan reads anyway. An object past it is refused from its
//     size alone, before any of it is read.
//   - SnapshotTotalBytes holds what the snapshot writes to the temporary directory, counting a blob once for
//     every path that names it. The per-object limit alone leaves the disk to the repository: a tree of many
//     objects each under it, or one blob named by many paths, writes as much as it names. 4 GiB is eight
//     times the working-set budget, above any repository whose whole tree is worth reading to scan its
//     source, and it is a bound on a disk, not on memory.
const (
	SnapshotWallTime    = 10 * time.Minute
	SnapshotObjectBytes = 512 << 20
	SnapshotTotalBytes  = 4 << 30
)

// snapshotTotal is what writeSnapshot may write in all. It is a variable so that a test can hold a snapshot
// to a small total; production code never assigns it.
var snapshotTotal int64 = SnapshotTotalBytes

// fileStreamBudget is what the --baseline reader (one long-lived `git cat-file --batch -Z` for the whole
// run) may use: a wall time for one request, so that the analysis between two requests is not counted, and
// the most bytes of one file it will hold. An object past the limit is discarded from the stream and
// reported as too large for that file; it is never allocated. The numbers are those of every other read of
// one file (the show row, revisionfs): the analyzers refuse source past 2 MiB and a policy past 1 MiB, so a
// file past 32 MiB is never one they read.
type fileStreamBudget struct {
	wall   time.Duration
	object int64
}

// fileReadBudget is a variable so that a test can hold the reader to a small one; production code never
// assigns it.
var fileReadBudget = fileStreamBudget{wall: metadataTimeout, object: fileBytes}

// snapshotBudget is the wall time of the git process that reads a snapshot's objects, and the most bytes of
// content one object may have.
type snapshotBudget struct {
	wall   time.Duration
	object int64
}

var defaultSnapshotBudget = snapshotBudget{wall: SnapshotWallTime, object: SnapshotObjectBytes}

// budgetFor is the budget of the git read that args name. It is a variable so that a test can hold a read
// to a short one; production code never assigns it.
var budgetFor = defaultBudgetFor

func defaultBudgetFor(args []string) budget {
	subcommand := ""
	if len(args) > 0 {
		subcommand = args[0]
	}
	switch subcommand {
	case "config", "rev-parse", "merge-base", "version", "cat-file":
		return budget{timeout: metadataTimeout, stdout: metadataBytes, stderr: gitStderrBudget}
	case "ls-tree":
		return budget{timeout: metadataTimeout, stdout: listingBytes, stderr: gitStderrBudget}
	case "show":
		return budget{timeout: metadataTimeout, stdout: fileBytes, stderr: gitStderrBudget}
	case "diff":
		return diffBudget(args)
	}
	return budget{timeout: metadataTimeout, stdout: unnamedBytes, stderr: gitStderrBudget}
}

// diffBudget separates the listing of every changed path (--name-status) from the diff of one path.
func diffBudget(args []string) budget {
	if slices.Contains(args, "--name-status") {
		return budget{timeout: diffListingTimeout, stdout: listingBytes, stderr: gitStderrBudget}
	}
	return budget{timeout: metadataTimeout, stdout: fileBytes, stderr: gitStderrBudget}
}
