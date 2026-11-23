package projectcheck

import (
	"slices"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// headCopy is what the analyzed revision holds at the policy's path.
type headCopy int

const (
	// headHoldsNoFile is nothing at the path, or a directory or a submodule: no file a policy could be.
	headHoldsNoFile headCopy = iota
	// headHoldsALink is a symbolic link or another entry that is not a regular file.
	headHoldsALink
	headHoldsAFile
)

// headEvidence is everything the warning about a policy on disk depends on.
type headEvidence struct {
	// unanswered is set when a question about the revision or the index could not be answered: HEAD's copy cannot be
	// read, git failed, or another git process holds the index.
	unanswered bool
	// conflicted is set when the index holds an unresolved conflict for the path.
	conflicted bool
	copy       headCopy
	// sameBytes is whether the file on disk is the revision's copy, once git's own conversions are applied.
	sameBytes bool
	// submoduleMessage is why the file lies inside a submodule checkout, or empty when it does not.
	submoduleMessage string
}

// warningCode is the warning that says how the policy on disk differs from the analyzed revision's, or empty for the
// same file. A policy that cannot be compared is never reported as a difference.
func (e headEvidence) warningCode() string {
	switch {
	case e.unanswered:
		return projectreadiness.WarnPolicyHeadUnreadable
	case e.conflicted, e.copy == headHoldsALink:
		return projectreadiness.WarnPolicyDiffersFromHead
	case e.copy == headHoldsNoFile:
		return projectreadiness.WarnPolicyNotCommitted
	case e.sameBytes:
		return ""
	}
	return projectreadiness.WarnPolicyDiffersFromHead
}

// readHeadEvidence asks git what the revision and the index hold for the selected policy's path. It never fails the
// check: a question git cannot answer is evidence that the two cannot be compared.
func readHeadEvidence(run gitrepo.Runner, dir, revision string, selection projectconfig.DiskSelection) headEvidence {
	path := selection.Path
	kind, err := gitrepo.EntryKindAtRevision(run, dir, revision, path)
	if err != nil {
		return headEvidence{unanswered: true}
	}
	evidence := headEvidence{copy: headCopyOf(kind)}
	if evidence.copy == headHoldsNoFile {
		message, cause, err := projectconfig.UncommittedPolicy(run, dir, revision, path)
		if err != nil {
			return headEvidence{unanswered: true}
		}
		if cause == projectreadiness.CauseInSubmodule {
			evidence.submoduleMessage = message
			return evidence
		}
	}
	if locked, err := indexIsLocked(run, dir); err != nil || locked {
		evidence.unanswered = true
		return evidence
	}
	indexed, err := gitrepo.IndexEntries(run, dir, path)
	if err != nil {
		evidence.unanswered = true
		return evidence
	}
	evidence.conflicted = slices.ContainsFunc(indexed.Entries, func(entry gitrepo.IndexEntry) bool { return entry.Stage != "0" })
	if evidence.copy == headHoldsAFile {
		evidence.sameBytes, evidence.unanswered = sameAsTheCommittedCopy(run, dir, revision, selection)
	}
	return evidence
}

func headCopyOf(kind gitrepo.EntryKind) headCopy {
	switch kind {
	case gitrepo.EntryRegularFile:
		return headHoldsAFile
	case gitrepo.EntrySymbolicLink, gitrepo.EntryOther:
		return headHoldsALink
	}
	return headHoldsNoFile
}
