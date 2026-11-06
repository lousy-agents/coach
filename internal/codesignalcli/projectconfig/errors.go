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

	// Missing says why a policy that is missing from disk is missing, for the
	// callers that have to tell the cases apart. It is MissingUnset for every
	// other kind. It is not PolicyCause, the stable word the readiness result carries.
	Missing MissingCause
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
	// PolicyCause is why no remedy fixes the gap, as a stable word, for an error that
	// offers none. It is empty when a restore or a new policy is offered.
	PolicyCause projectreadiness.PolicyCause
	// GitFailure is set when git could not answer a question about the policy,
	// so a caller that reports operational failures separately can return it.
	GitFailure error
}

// MissingCause is why a policy named with --project-config is missing from disk.
type MissingCause int

const (
	// MissingUnset is the zero value, for an error that is not a missing policy.
	MissingUnset MissingCause = iota
	// MissingAbsent means nothing is at the path.
	MissingAbsent
	// MissingBlockedParent means a parent path component is a file, so the path
	// cannot be created or restored while that file stays.
	MissingBlockedParent
)

func (e *ConfigError) Error() string { return e.Message }

// OffersAuthoring is whether a new policy is the way out of this error. It is the one decision every
// gate that starts or suggests guided authoring asks, and the remedy makes it: the kind says what went
// wrong, and a refusal that shares a kind with an invalid document (a policy committed as a link, a path
// the disk cannot use) offers no new policy.
func (e *ConfigError) OffersAuthoring() bool {
	action, named := projectreadiness.NextActionForRemedy(e.Remedy)
	return named && action == projectreadiness.NextActionAuthorPolicy
}

// readError classifies a runProjectConfigGit failure into a
// user-facing message that never surfaces raw git stderr. A
// *gitrepo.BoundError is our own timeout/output-budget text and is
// safe to include verbatim. Any other failure means git itself reported the
// read as failed; that case is further split by whether repoPath exists in
// the worktree, so a user who forgot to commit a generated config is told to
// commit it rather than shown a generic not-found message.
func readError(run gitrepo.Runner, dir, revision, repoPath string, gitErr error) error {
	var boundErr *gitrepo.BoundError
	if errors.As(gitErr, &boundErr) {
		remedy, cause := boundOutcome(boundErr)
		return &ConfigError{Kind: KindUnreadable, Remedy: remedy, PolicyCause: cause, Message: fmt.Sprintf("coach codesignal: --project-config %q is not readable at revision %q (project_config_invalid): %s", repoPath, revision, boundErr.Error())}
	}
	exists, err := configExistsInWorktree(run, dir, repoPath)
	if err != nil {
		return unansweredRead(repoPath, revision, newGitFailure("whether it exists in the worktree", err))
	}
	if exists {
		message, cause, _ := UncommittedPolicy(run, dir, revision, repoPath)
		return &ConfigError{Kind: KindUncommitted, Remedy: projectreadiness.RemedyNone, PolicyCause: cause, Message: message}
	}
	return &ConfigError{Kind: KindNotFound, Remedy: projectreadiness.RemedyAuthor, Message: fmt.Sprintf("coach codesignal: --project-config %q was not found at revision %q (project_config_invalid)", repoPath, revision)}
}

// unansweredRead is a policy that cannot be told from absent because git could not answer a question
// about it. It names the question and offers nothing: "was not found" would offer a new policy over a
// file that may be there.
func unansweredRead(repoPath, revision string, failure *gitFailure) error {
	return &ConfigError{Kind: KindUnreadable, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseGitUnreadable, Message: fmt.Sprintf("coach codesignal: --project-config %q is not readable at revision %q (project_config_invalid): Coach could not check %s, so it gives no advice", repoPath, revision, failure.describe())}
}

// boundOutcome is the way out of a read that tripped a bound, or the reason it has none, decided once
// so the two cannot disagree. A policy over the size budget is a content gap a smaller policy fixes, so
// authoring stays offered, as --check-project already reports it, and there is no cause. A time or
// stderr bound says the read failed for a reason a new policy does not change: no remedy, and the
// cause says the read failed.
func boundOutcome(boundErr *gitrepo.BoundError) (projectreadiness.PolicyRemedy, projectreadiness.PolicyCause) {
	if boundErr.Kind == gitrepo.BoundStdout {
		return projectreadiness.RemedyAuthor, ""
	}
	return projectreadiness.RemedyNone, projectreadiness.CauseGitUnreadable
}

// configExistsInWorktree reports whether repoPath is readable in the worktree at dir, which it
// finds through run. Any stat failure (not just "does not exist") is treated as absent, since
// distinguishing permission-denied from a missing file has no observer here. Where git cannot say
// where the worktree is, it is an error: the caller must not read that as absent.
func configExistsInWorktree(run gitrepo.Runner, dir, repoPath string) (bool, error) {
	root, err := gitrepo.RepositoryRootWith(run, dir)
	if err != nil {
		return false, err
	}
	return worktreeHolds(root, repoPath), nil
}

// worktreeHolds is whether repoPath is readable under the worktree rooted at root.
func worktreeHolds(root, repoPath string) bool {
	_, statErr := os.Stat(filepath.Join(root, repoPath))
	return statErr == nil
}

func missingError(repoPath string) error {
	return &ConfigError{Kind: KindMissing, Missing: MissingAbsent, Remedy: projectreadiness.RemedyAuthor, Message: fmt.Sprintf("coach codesignal: --project-config %q was not found on disk at the repository root (project_config_missing); the path is repository-relative", repoPath)}
}

// blockedParentError is a missing policy whose path cannot be created because a
// parent component is not a directory, so writing a new policy there is not a
// way forward, and neither is restoring the committed one: git restore would
// delete the file in the way. It is still missing, never a reason to read
// another policy.
func blockedParentError(repoPath, component string) error {
	return &ConfigError{Kind: KindMissing, Missing: MissingBlockedParent, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseParentNotDirectory, Message: fmt.Sprintf("coach codesignal: --project-config %q was not found on disk at the repository root (project_config_missing): a parent path component is not a directory, %q; move or remove it; the path is repository-relative", repoPath, component)}
}

// unusableOnDiskError is a policy path the working tree decided is unusable. A
// new policy cannot replace what is there, so authoring is not offered.
func unusableOnDiskError(repoPath string, cause projectreadiness.PolicyCause, reason string) error {
	return newUnusableOnDisk(repoPath, cause, reason)
}

// newUnusableOnDisk is unusableOnDiskError as the concrete type. The cause says which fix the
// path needs, so a reader can tell a directory from a link, an unreadable file and a name in
// another case without parsing the message.
func newUnusableOnDisk(repoPath string, cause projectreadiness.PolicyCause, reason string) *ConfigError {
	return &ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyNone, PolicyCause: cause, Message: fmt.Sprintf("coach codesignal: --project-config %q %s (project_config_invalid)", repoPath, reason)}
}

// invalidPathError is a --project-config value that cannot name a policy in this repository. A new policy
// is written at its own location and leaves the path wrong, so authoring is not offered; the reason says what
// is wrong with the path.
func invalidPathError(repoPath, revision, reason string) error {
	return &ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyNone, PolicyCause: projectreadiness.CauseInvalidPath, Message: fmt.Sprintf("coach codesignal: --project-config %q is invalid at revision %q (project_config_invalid): %s", repoPath, revision, reason)}
}

func invalidError(repoPath, revision, reason string) error {
	return &ConfigError{Kind: KindInvalid, Remedy: projectreadiness.RemedyAuthor, Message: fmt.Sprintf("coach codesignal: --project-config %q is invalid at revision %q (project_config_invalid): %s", repoPath, revision, reason)}
}
