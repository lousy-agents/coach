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
	// it. The zero value offers nothing, so a constructor that left it unset
	// would withhold advice, never suggest a new policy where one cannot help.
	Remedy projectreadiness.PolicyRemedy
	// GitFailure is set when git could not answer a question about the policy,
	// so a caller that reports operational failures separately can return it.
	GitFailure error
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
		return &ConfigError{Kind: KindUncommitted, Remedy: projectreadiness.RemedyNone, Message: UncommittedPolicyMessage(revision, repoPath)}
	}
	return &ConfigError{Kind: KindNotFound, Remedy: projectreadiness.RemedyAuthor, Message: fmt.Sprintf("coach codesignal: --project-config %q was not found at revision %q (project_config_invalid)", repoPath, revision)}
}

// UncommittedPolicyMessage is what the scan says about a --project-config policy
// that exists in the working tree but is not committed at the analyzed revision.
func UncommittedPolicyMessage(revision, repoPath string) string {
	return fmt.Sprintf("coach codesignal: --project-config %q exists in the worktree but is not committed at revision %q (project_config_invalid): commit the file so it is readable at the analyzed revision", repoPath, revision)
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
