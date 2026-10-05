package projectconfig

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
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
)

// Error signals a --project-config value that is missing,
// unreadable, or does not satisfy the frozen v1 schema. It maps to exit code
// 2 and is reported as a single stderr message; no report is written to
// stdout.
type ConfigError struct {
	Message string
	Kind    ErrorKind
}

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
		return &ConfigError{Kind: KindUnreadable, Message: fmt.Sprintf("coach codesignal: --project-config %q is not readable at revision %q (project_config_invalid): %s", repoPath, revision, boundErr.Error())}
	}
	if configExistsInWorktree(dir, repoPath) {
		return &ConfigError{Kind: KindUncommitted, Message: fmt.Sprintf("coach codesignal: --project-config %q exists in the worktree but is not committed at revision %q (project_config_invalid): commit the file so it is readable at the analyzed revision", repoPath, revision)}
	}
	return &ConfigError{Kind: KindNotFound, Message: fmt.Sprintf("coach codesignal: --project-config %q was not found at revision %q (project_config_invalid)", repoPath, revision)}
}

// configExistsInWorktree reports whether repoPath is readable in the
// worktree at dir. Any stat failure (not just "does not exist") is treated
// as absent: this function's only caller already falls back to a generic
// not-found-at-revision message in that case, so distinguishing
// permission-denied or other I/O errors from a missing file has no observer.
func configExistsInWorktree(dir, repoPath string) bool {
	_, statErr := os.Stat(filepath.Join(dir, repoPath))
	return statErr == nil
}

func invalidError(repoPath, revision, reason string) error {
	return &ConfigError{Kind: KindInvalid, Message: fmt.Sprintf("coach codesignal: --project-config %q is invalid at revision %q (project_config_invalid): %s", repoPath, revision, reason)}
}
