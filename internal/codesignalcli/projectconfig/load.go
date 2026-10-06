package projectconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
)

// runProjectConfigGit is the Git seam used by Load. Tests may
// replace it to exercise timeout and bound failures without hanging.
var runProjectConfigGit = func(dir string, args ...string) ([]byte, error) {
	return gitrepo.RunBytesBounded(dir, MaxBytes, MaxGitStderr, GitTimeout, args...)
}

// LoadForReadiness reads and validates repoPath at revision
// like Load, but keeps a git-read failure distinct from a
// content/schema rejection instead of collapsing both into
// *ConfigError: checkPolicy must report the former as an
// *gitrepo.OperationalError (exit 1, fail closed) and only the latter as the
// policy_invalid gap. It returns the decoded Config directly so
// checkPolicy needs no second decode of the same bytes.
//
// A stdout-size-budget failure is deliberately classified as content
// rejection (policy_invalid, exit 0) rather than operational (exit 1),
// diverging from Load's readError, which reports
// every *gitrepo.BoundError -- including this same size-budget case --
// as project_config_invalid at exit 2. The committed policy file's size is
// something its author controls and can fix, unlike a corrupt object store
// or a timed-out git process, so --check-project's read-only, actionable-gap
// contract treats it as a gap rather than an environment failure. A timeout
// or stderr-budget failure still reports *gitrepo.OperationalError: those indicate a
// resource/environment condition, not a defect in the file's content.
func LoadForReadiness(dir, revision, repoPath string) (Config, error) {
	if err := ValidatePath(repoPath); err != nil {
		return Config{}, invalidError(repoPath, revision, err.Error())
	}

	data, err := runProjectConfigGit(dir, "show", revision+":"+repoPath)
	if err != nil {
		var boundErr *gitrepo.BoundError
		if errors.As(err, &boundErr) && boundErr.Kind == gitrepo.BoundStdout {
			return Config{}, invalidError(repoPath, revision, fmt.Sprintf("committed content exceeds the %d-byte size budget: %s", MaxBytes, err))
		}
		return Config{}, &gitrepo.OperationalError{Message: fmt.Sprintf("coach codesignal --check-project: --project-config %q could not be read at revision %q: %s", repoPath, revision, err)}
	}

	config, err := Parse(data)
	if err != nil {
		return Config{}, invalidError(repoPath, revision, err.Error())
	}
	return config, nil
}

// Load reads and validates a repository-relative config at an
// immutable Git revision. Reading through Git, rather than the worktree,
// keeps a diff report from mixing committed source facts with uncommitted
// configuration. Git stdout/stderr, wall time, document size, and JSON
// nesting are bounded at this boundary.
func Load(dir, revision, repoPath string) (json.RawMessage, error) {
	if err := ValidatePath(repoPath); err != nil {
		return nil, invalidError(repoPath, revision, err.Error())
	}

	data, err := runProjectConfigGit(dir, "show", revision+":"+repoPath)
	if err != nil {
		return nil, readError(dir, revision, repoPath, err)
	}
	if err := validateJSON(data); err != nil {
		return nil, invalidError(repoPath, revision, err.Error())
	}
	return json.RawMessage(data), nil
}

// ValidatePath validates a --project-config value's shape using
// the same rules Load enforces, without touching Git or the
// filesystem. --check-project's argument validation calls this before any
// readiness check runs, so an invalid path (absolute, containing "..", or
// using a backslash separator) is rejected at argument time rather than
// surfacing as a false policy_missing/policy_invalid readiness gap.
func ValidatePath(repoPath string) error {
	if repoPath == "" || path.IsAbs(repoPath) {
		return fmt.Errorf("path must be a non-empty repository-relative path")
	}
	clean := path.Clean(repoPath)
	if clean != repoPath || clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("path must be normalized and remain inside the repository")
	}
	if strings.Contains(repoPath, "\\") {
		return fmt.Errorf("path must use repository-relative slash separators")
	}
	return nil
}
