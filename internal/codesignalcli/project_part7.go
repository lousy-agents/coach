package codesignalcli

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

// LoadProjectConfig reads and validates a repository-relative config at an
// immutable Git revision. Reading through Git, rather than the worktree,
// keeps a diff report from mixing committed source facts with uncommitted
// configuration. Git stdout/stderr, wall time, document size, and JSON
// nesting are bounded at this boundary.
func LoadProjectConfig(dir, revision, repoPath string) (json.RawMessage, error) {
	if err := validateProjectConfigPath(repoPath); err != nil {
		return nil, projectConfigError(repoPath, revision, err.Error())
	}

	data, err := runProjectConfigGit(dir, "show", revision+":"+repoPath)
	if err != nil {
		return nil, projectConfigGitError(dir, revision, repoPath, err)
	}
	if err := validateProjectConfigJSON(data); err != nil {
		return nil, projectConfigError(repoPath, revision, err.Error())
	}
	return json.RawMessage(data), nil
}

func validateProjectConfigCrossFields(config projectConfig, seenLayerNames map[string]struct{}) error {
	if config.SourceSinkPack != "" && config.SourceSinkPack != "builtin-v1" {
		return fmt.Errorf("source_sink_pack must be \"builtin-v1\" when supplied")
	}
	if config.RequiredLayer != "" {
		if _, ok := seenLayerNames[config.RequiredLayer]; !ok {
			return fmt.Errorf("required_layer references undefined layer %q", config.RequiredLayer)
		}
	}
	return nil
}

func validateProjectConfigPath(repoPath string) error {
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

// ResolveProjectBackend reports whether a project-analysis backend is
// registered for language. "go" and "typescript" both have registered
// backends today; every other language, including the empty string, remains
// unavailable until its own backend lands.
func ResolveProjectBackend(language string) error {
	if language == "go" || language == "typescript" {
		return nil
	}
	return &ProjectBackendUnavailableError{Message: fmt.Sprintf("coach codesignal: no project-analysis backend is available for language %q yet (project_backend_unavailable)", language)}
}
