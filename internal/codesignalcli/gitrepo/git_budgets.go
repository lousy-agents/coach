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

// The numbers follow the bounds the policy and snapshot reads already run under (30 s, a 64 MiB listing, a
// 32 MiB file, 64 KiB of stderr), which revisionfs takes from here.
const (
	gitStderrBudget = 64 << 10
	// A question git answers from a few files under .git: whether this is a worktree, what HEAD is, whether
	// two commits share a base, how the repository is configured, which version git is, what the index holds
	// for one policy path. Any of them taking
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
//   - snapshotWallTime holds the whole stream. 10 minutes is long for any repository this fits, and a git that
//     stops answering does not hold the scan past it.
//   - snapshotListingWallTime holds the listing of the tree that names the files. It is read as a stream, a record
//     at a time, so it has no limit on its size: a limit would refuse the repository with the most files, and
//     v0.11.0, which read the tree with git archive, had none. Two minutes is the bound of the other read that
//     walks a whole tree (the listing of the missing objects, diffListingTimeout). git lists about 180 MB of
//     records a second (a tree of 300000 files, a 41 MB listing, in 0.23 s), so it is room for a listing of
//     twenty gigabytes, which is tens of millions of files, and a git that has not finished by then is not
//     listing.
//   - snapshotObjectBytes holds one object in memory: an object within it is held, and written to disk before
//     the next is read. 32 MiB is the limit of every other read of one file (the show row, revisionfs). An
//     object past it is never held. When a path that names it is a file an analyzer reads (a source, a Go
//     module file, JSON), it is written to disk a chunk at a time and in full, because `go list` parses the
//     file before any analyzer refuses it: the analyzers refuse source past 2 MiB and a policy past 1 MiB, but
//     an empty file in its place fails `go list` with a parse error of its own ("expected 'package', found
//     'EOF'"), where v0.11.0 went on and the analyzer named the file too large. Any other object is read past
//     without being held, and its paths get a zero-length file.
//   - snapshotTotalBytes holds what the snapshot writes to the temporary directory, counting a blob once for
//     every path that names it, whether it is held or read to disk in chunks. The per-object limit alone leaves the disk to the repository: a tree of
//     many objects each under it, or one blob named by many paths, writes as much as it names. 4 GiB is
//     128 objects at the limit and is a bound on a disk, not on memory. Past it, the files the analyzers
//     read are still written (they are written first) and the others become zero-length files; the
//     snapshot fails only when the files the analyzers read pass it by themselves.
const (
	snapshotListingWallTime = diffListingTimeout
	snapshotWallTime        = 10 * time.Minute
	snapshotObjectBytes     = 32 << 20
	snapshotTotalBytes      = 4 << 30
)

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

// defaultFileReadBudget is what the --baseline reader is held to. The reader takes its budget as an argument
// (newRevisionFileReader), so a test hands in a small one.
func defaultFileReadBudget() fileStreamBudget {
	return fileStreamBudget{wall: metadataTimeout, object: fileBytes}
}

// snapshotBudget is the wall time of the git process that reads a snapshot's objects, and the most bytes of
// content one object may have.
type snapshotBudget struct {
	wall   time.Duration
	object int64
}

// defaultSnapshotBudget is what a real snapshot is read under (defaultSnapshotWrite). A test hands in its own.
func defaultSnapshotBudget() snapshotBudget {
	return snapshotBudget{wall: snapshotWallTime, object: snapshotObjectBytes}
}

// The reads below name a budget of their own, because the subcommand does not say what they hold: a small
// committed file that must fail closed, a status listing, the root of a worktree. They are held by a
// BoundedRunner that the package making the read builds from these numbers, and the numbers are here so that
// this file is the one place that holds them. A package aliases them (projectconfig.MaxBytes is
// CommittedFileBytes), and a test pins each one.
const (
	// A policy document, a manifest or a lockfile at a revision. 1 MiB is the size limit of a policy document:
	// a committed file past it fails in under a second, not after a 30 s read.
	CommittedFileBytes   = 1 << 20
	CommittedFileTimeout = metadataTimeout
	// GitStderrBytes is the stderr every one of these reads keeps.
	GitStderrBytes = gitStderrBudget

	// The listing of a whole tree at a revision and one tracked file's content: the reads revisionfs makes
	// to give the analyzers a revision as a file system.
	RevisionListingBytes = listingBytes
	RevisionFileBytes    = fileBytes
	RevisionReadTimeout  = metadataTimeout

	// git status of a working tree. The listing prints a line for every changed path, so it has the largest
	// output of the named reads.
	WorktreeStatusBytes   = 16 << 20
	WorktreeStatusTimeout = metadataTimeout

	// The status listing of one working directory in the TypeScript setup, not a whole-tree read.
	SetupResidueBytes   = 1 << 20
	SetupResidueTimeout = 10 * time.Second

	// rev-parse --show-toplevel for the walk ceiling of nearest-package.json: one path.
	WorktreeRootBytes   = 4 << 10
	WorktreeRootStderr  = 4 << 10
	WorktreeRootTimeout = 10 * time.Second
)

// defaultBudgetFor is the budget of the git read that args name.
func defaultBudgetFor(args []string) budget {
	subcommand := ""
	if len(args) > 0 {
		subcommand = args[0]
	}
	switch subcommand {
	case "config", "rev-parse", "merge-base", "version", "cat-file", "ls-files":
		return budget{timeout: metadataTimeout, stdout: metadataBytes, stderr: gitStderrBudget}
	case "ls-tree":
		return budget{timeout: metadataTimeout, stdout: listingBytes, stderr: gitStderrBudget}
	case "show":
		return budget{timeout: metadataTimeout, stdout: fileBytes, stderr: gitStderrBudget}
	case "diff":
		return diffBudget(args)
	case "rev-list":
		// the partial-clone guard lists the missing objects of a whole tree
		return budget{timeout: diffListingTimeout, stdout: listingBytes, stderr: gitStderrBudget}
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
