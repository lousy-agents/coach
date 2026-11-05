package projectconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// ErrorKind discriminates why a *ConfigError was
// produced, distinct from its human-facing Message (which stays frozen). A
// caller that needs to react differently to "no policy was ever committed"
// than to a usage error, an uncommitted-in-worktree file, or a transient git
// failure reads Kind rather than pattern-matching Message.
type ErrorKind int

const (
	// KindUnset is the zero value: a *ConfigError whose
	// cause was never classified. It is deliberately non-actionable, so an
	// untagged construction site cannot alias a specific kind.
	KindUnset ErrorKind = iota
	// KindNotFound means repoPath does not exist at revision and is
	// not present in the worktree either: no policy was ever authored or
	// committed.
	KindNotFound
	// KindInvalid means repoPath itself failed shape validation, or
	// content read at revision failed to parse or validate against the
	// schema.
	KindInvalid
	// KindUncommitted means repoPath exists in the worktree but not
	// at the analyzed revision.
	KindUncommitted
	// KindUnreadable means a gitrepo.RunBytesBoundedWith timeout or
	// output-budget bound tripped while reading repoPath: a transient
	// operational condition, not a defect in the file's content.
	KindUnreadable
	// KindMissing means an explicit --project-config names a file that does
	// not exist on disk. It never falls back to another policy.
	KindMissing
)

// Error signals a --project-config value that is missing,
// unreadable, or does not satisfy the frozen v1 schema. It maps to exit code
// 2 and is reported as a single stderr message; no report is written to
// stdout.
type ConfigError struct {
	Message string
	Kind    ErrorKind

	// Cause says why a policy that is missing from disk is missing, for the
	// callers that have to tell the cases apart. It is CauseUnset for every
	// other kind.
	Cause MissingCause
	// FromWorkingTree is set when the working tree, not the analyzed revision,
	// produced this error. A caller that holds it does not need to look at the
	// working tree for the same policy again.
	FromWorkingTree bool
	// UnreadStagedEdit is set when the working tree could not supply the
	// policy and the policy is committed, and the index holds an edit to it that
	// nothing has read, or git could not say it holds none. A report that rests
	// on this error does not reflect that edit.
	UnreadStagedEdit bool
	// Remedy is the way out the tool offers. Every constructor in this file sets
	// it, because the zero value means authoring and a constructor that left it
	// unset would offer a new policy where one cannot help.
	Remedy projectreadiness.PolicyRemedy
}

// MissingCause is why a policy named with --project-config is missing from disk.
type MissingCause int

const (
	// CauseUnset is the zero value, for an error that is not a missing policy.
	CauseUnset MissingCause = iota
	// CauseAbsent means nothing is at the path.
	CauseAbsent
	// CauseBlockedParent means a parent path component is a file, so the path
	// cannot be created or restored while that file stays.
	CauseBlockedParent
)

func (e *ConfigError) Error() string { return e.Message }

// readError classifies a runProjectConfigGit failure into a
// user-facing message that never surfaces raw git stderr. A
// *gitrepo.BoundError is our own timeout/output-budget text and is
// safe to include verbatim. Any other failure means git itself reported the
// read as failed; that case is further split by whether repoPath exists in
// the worktree, so a user who forgot to commit a generated config is told to
// commit it rather than shown a generic not-found message.
func readError(dir, revision, repoPath string, gitErr error) error {
	var boundErr *gitrepo.BoundError
	if errors.As(gitErr, &boundErr) {
		return &ConfigError{Kind: KindUnreadable, Remedy: boundRemedy(boundErr), Message: fmt.Sprintf("coach codesignal: --project-config %q is not readable at revision %q (project_config_invalid): %s", repoPath, revision, boundErr.Error())}
	}
	if configExistsInWorktree(dir, repoPath) {
		return &ConfigError{Kind: KindUncommitted, Remedy: projectreadiness.RemedyNone, Message: fmt.Sprintf("coach codesignal: --project-config %q exists in the worktree but is not committed at revision %q (project_config_invalid): commit the file so it is readable at the analyzed revision", repoPath, revision)}
	}
	return &ConfigError{Kind: KindNotFound, Remedy: projectreadiness.RemedyAuthor, Message: fmt.Sprintf("coach codesignal: --project-config %q was not found at revision %q (project_config_invalid)", repoPath, revision)}
}

// boundRemedy is the way out of a read that tripped a bound. A policy over the
// size budget is a content gap a smaller policy fixes, so authoring stays
// offered, as --check-project already reports it. A time or stderr bound says
// the read failed for a reason a new policy does not change.
func boundRemedy(boundErr *gitrepo.BoundError) projectreadiness.PolicyRemedy {
	if boundErr.Kind == gitrepo.BoundStdout {
		return projectreadiness.RemedyAuthor
	}
	return projectreadiness.RemedyNone
}

// configExistsInWorktree reports whether repoPath is readable in the
// worktree at dir. Any stat failure (not just "does not exist") is treated
// as absent: this function's only caller already falls back to a generic
// not-found-at-revision message in that case, so distinguishing
// permission-denied or other I/O errors from a missing file has no observer.
func configExistsInWorktree(dir, repoPath string) bool {
	root, err := gitrepo.RepositoryRoot(dir)
	if err != nil {
		return false
	}
	_, statErr := os.Stat(filepath.Join(root, repoPath))
	return statErr == nil
}

func missingError(repoPath string) error {
	return &ConfigError{Kind: KindMissing, Cause: CauseAbsent, Remedy: projectreadiness.RemedyAuthor, Message: fmt.Sprintf("coach codesignal: --project-config %q was not found on disk at the repository root (project_config_missing); the path is repository-relative", repoPath)}
}

// restoreOffer is what is added to a missing policy that is committed at the
// analyzed revision: the way out, if any, and the sentence that says so. Each
// one withholds the restore command when running it would overwrite something
// or recreate what the scan refuses, and none offers a new policy, because the
// policy is committed already.
type restoreOffer struct {
	remedy projectreadiness.PolicyRemedy
	text   string
}

// restorableOffer brings the committed policy back with the restore command.
func restorableOffer(revision, repoPath string) restoreOffer {
	return restoreOffer{remedy: projectreadiness.RemedyRestore, text: ". It is committed at the analyzed revision: " + RestoreAdvice(revision, repoPath)}
}

// undecidableOffer is for a policy that git could not say is committed. Offering
// a restore could overwrite, and offering authoring could replace a reviewed
// policy, so neither is offered.
var undecidableOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	text:   ". Coach could not check whether it is committed at the analyzed revision, so it offers neither a restore nor a new policy; resolve the git error and run again",
}

// stagedEditOffer is for a committed policy whose index entry holds an edit. The
// restore command resets the index entry as well, which would discard the edit,
// and so would restoring from git after a blocking file is moved.
var stagedEditOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	text:   ". It is committed at the analyzed revision, and the index holds a staged edit to it that a restore would discard, so none is offered; review the staged edit with git diff --cached, keep what you want, then run again",
}

// committedLinkOffer is for a policy committed as a symbolic link. Restoring it
// would recreate a link the scan refuses, and a new file would not be the
// committed policy.
var committedLinkOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	text:   ". It is committed at the analyzed revision as a symbolic link, which a policy cannot be, so there is nothing to restore; name another policy or commit a regular file at this path",
}

// sparseCheckoutOffer is for a committed policy that the sparse checkout leaves
// out of the working tree. git restore rejects a path the checkout skips, so the
// command would fail, and the way back is to widen the checkout.
var sparseCheckoutOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	text:   ". It is committed at the analyzed revision, but this sparse checkout leaves it out of the working tree, so a restore cannot bring it back; add its directory to the sparse checkout, then run again",
}

// committedBlockedParentOffer names no restore command, because running one
// before the file is moved would delete the file.
var committedBlockedParentOffer = restoreOffer{
	remedy: projectreadiness.RemedyNone,
	text:   ". The policy is committed at the analyzed revision, so after moving the file, restore the policy from git.",
}

// annotated is found with the offer applied.
func annotated(found *ConfigError, offer restoreOffer) *ConfigError {
	result := *found
	result.Remedy = offer.remedy
	result.Message += offer.text
	return &result
}

// blockedParentError is a missing policy whose path cannot be created because a
// parent component is not a directory, so writing a new policy there is not a
// way forward, and neither is restoring the committed one: git restore would
// delete the file in the way. It is still missing, never a reason to read
// another policy.
func blockedParentError(repoPath, component string) error {
	return &ConfigError{Kind: KindMissing, Cause: CauseBlockedParent, Remedy: projectreadiness.RemedyNone, Message: fmt.Sprintf("coach codesignal: --project-config %q was not found on disk at the repository root (project_config_missing): a parent path component is not a directory, %q; move or remove it; the path is repository-relative", repoPath, component)}
}

// unusableOnDiskError is a policy path the working tree decided is unusable. A
// new policy cannot replace what is there, so authoring is not offered.
func unusableOnDiskError(repoPath, reason string) error {
	return &ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyNone, Message: fmt.Sprintf("coach codesignal: --project-config %q %s (project_config_invalid)", repoPath, reason)}
}

func invalidError(repoPath, revision, reason string) error {
	return &ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyAuthor, Message: fmt.Sprintf("coach codesignal: --project-config %q is invalid at revision %q (project_config_invalid): %s", repoPath, revision, reason)}
}
