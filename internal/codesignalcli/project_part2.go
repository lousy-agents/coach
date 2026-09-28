package codesignalcli

import (
	"errors"
	"fmt"

	"os"

	"path/filepath"

	"time"
)

// validateProjectConfigLayers returns the declared layer names so
// validateProjectConfigForbiddenImports and validateProjectConfigCrossFields
// can check their own layer references against it.
func validateProjectConfigLayers(layers []projectConfigLayer) (map[string]struct{}, error) {
	seenLayerNames := make(map[string]struct{}, len(layers))
	var allPrefixes []string
	for _, layer := range layers {
		if layer.Name == "" {
			return nil, fmt.Errorf("layer name must be non-empty")
		}
		if _, exists := seenLayerNames[layer.Name]; exists {
			return nil, fmt.Errorf("layer names must be unique")
		}
		seenLayerNames[layer.Name] = struct{}{}
		if len(layer.Prefixes) == 0 {
			return nil, fmt.Errorf("layer %q must contain at least one prefix", layer.Name)
		}
		for _, prefix := range layer.Prefixes {
			if err := validateProjectConfigDirectory(prefix); err != nil {
				return nil, fmt.Errorf("layer %q prefix %q: %s", layer.Name, prefix, err)
			}
			allPrefixes = append(allPrefixes, prefix)
		}
	}
	if len(allPrefixes) > maxProjectConfigLayerPrefixes {
		return nil, fmt.Errorf("layer prefixes exceed budget of %d entries", maxProjectConfigLayerPrefixes)
	}
	if hasDuplicateOrOverlappingPaths(allPrefixes) {
		return nil, fmt.Errorf("layer prefixes must be unique and non-overlapping")
	}
	return seenLayerNames, nil
}

// projectConfigGitError classifies a runProjectConfigGit failure into a
// user-facing message that never surfaces raw git stderr. A
// *gitOperationalBoundError is our own timeout/output-budget text and is
// safe to include verbatim. Any other failure means git itself reported the
// read as failed; that case is further split by whether repoPath exists in
// the worktree, so a user who forgot to commit a generated config is told to
// commit it rather than shown a generic not-found message.
func projectConfigGitError(dir, revision, repoPath string, gitErr error) error {
	var boundErr *gitOperationalBoundError
	if errors.As(gitErr, &boundErr) {
		return &ProjectConfigError{Kind: ProjectConfigUnreadable, Message: fmt.Sprintf("coach codesignal: --project-config %q is not readable at revision %q (project_config_invalid): %s", repoPath, revision, boundErr.Error())}
	}
	if configExistsInWorktree(dir, repoPath) {
		return &ProjectConfigError{Kind: ProjectConfigUncommitted, Message: fmt.Sprintf("coach codesignal: --project-config %q exists in the worktree but is not committed at revision %q (project_config_invalid): commit the file so it is readable at the analyzed revision", repoPath, revision)}
	}
	return &ProjectConfigError{Kind: ProjectConfigNotFound, Message: fmt.Sprintf("coach codesignal: --project-config %q was not found at revision %q (project_config_invalid)", repoPath, revision)}
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

// runGitBytesBounded runs git with a wall-time limit and hard caps on
// collected stdout and stderr, building the child via the package's default
// gitCommandContext seam. The LimitReader stops after maxStdout+1 bytes so
// an oversized blob is detected without buffering the entire child output.
func runGitBytesBounded(dir string, maxStdout, maxStderr int64, timeout time.Duration, args ...string) ([]byte, error) {
	return runGitBytesBoundedWith(gitCommandContext, dir, maxStdout, maxStderr, timeout, args...)
}
func (e *gitOperationalBoundError) Error() string { return e.message }
func projectConfigError(repoPath, revision, reason string) error {
	return &ProjectConfigError{Kind: ProjectConfigInvalid, Message: fmt.Sprintf("coach codesignal: --project-config %q is invalid at revision %q (project_config_invalid): %s", repoPath, revision, reason)}
}
func (e *ProjectConfigError) Error() string { return e.Message }
func validateProjectConfigJSON(data []byte) error {
	_, err := parseProjectConfig(data)
	return err
}
func (e *ProjectBackendUnavailableError) Error() string { return e.Message }
