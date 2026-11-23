package projectconfig

import (
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// CommittedCopyKind is what the revision holds at a path, as far as comparing it with the policy on disk needs.
type CommittedCopyKind int

const (
	// CopyAbsent is nothing at the path.
	CopyAbsent CommittedCopyKind = iota
	// CopyReadable is a regular file whose bytes were read in full, whether or not they are a usable policy.
	CopyReadable
	// CopyNotAFile is a link, directory or submodule: a revision entry that cannot be a policy.
	CopyNotAFile
	// CopyUnreadable is a regular file whose blob could not be read in full within the budget.
	CopyUnreadable
)

// CommittedCopy is what the revision holds at a policy path, as data and never as a refusal: whether that copy is a
// usable policy is no concern of a scan that uses the policy on disk.
type CommittedCopy struct {
	Kind    CommittedCopyKind
	Content []byte
	// Bound is the bound of git that stopped the read of an unreadable blob. It is CopyBoundNone for every other copy.
	Bound CopyBound
}

// CopyBound is the bound of git that tripped while reading one committed blob.
type CopyBound int

const (
	CopyBoundNone CopyBound = iota
	CopyBoundTime
	CopyBoundStderr
)

// ReadCommittedCopy reads what revision holds at repoPath. A blob that is over the size budget or that git cannot read
// is CopyUnreadable. A failure to say what the path holds at all, or a timeout, is returned: it describes the
// repository and not the copy, so it is no disclosure.
func ReadCommittedCopy(dir, revision, repoPath string) (CommittedCopy, error) {
	return readCommittedCopyThrough(NewGitRunner(), dir, revision, repoPath)
}

func readCommittedCopyThrough(run gitrepo.Runner, dir, revision, repoPath string) (CommittedCopy, error) {
	kind, err := gitrepo.EntryKindAtRevision(run, dir, revision, repoPath)
	if err != nil {
		return CommittedCopy{}, err
	}
	switch kind {
	case gitrepo.EntryAbsent:
		return CommittedCopy{Kind: CopyAbsent}, nil
	case gitrepo.EntryRegularFile:
		return readCommittedBlob(run, dir, revision, repoPath)
	}
	return CommittedCopy{Kind: CopyNotAFile}, nil
}

func readCommittedBlob(run gitrepo.Runner, dir, revision, repoPath string) (CommittedCopy, error) {
	data, err := run(dir, "show", revision+":"+repoPath)
	var bound *gitrepo.BoundError
	switch {
	case err == nil:
		return CommittedCopy{Kind: CopyReadable, Content: data}, nil
	case !errors.As(err, &bound) || bound.Kind == gitrepo.BoundStdout:
		return CommittedCopy{Kind: CopyUnreadable}, nil
	}
	return CommittedCopy{}, &gitrepo.OperationalError{Message: fmt.Sprintf("coach codesignal: %q could not be read at revision %q: %s", repoPath, revision, err), Cause: err}
}
