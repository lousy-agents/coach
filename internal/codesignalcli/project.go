package codesignalcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"path"
	"sort"
	"strings"
	"time"
)

// Project-config boundary budgets. Config is repository-controlled input and
// must fail closed before unbounded memory/CPU or a hung git child can stall
// the CLI (issue #208 finite project-phase resource behavior).
const (
	maxProjectConfigBytes     = 1 << 20 // 1 MiB
	maxProjectConfigJSONDepth = 32
	maxProjectConfigGitStderr = 64 << 10 // 64 KiB
	projectConfigGitTimeout   = 30 * time.Second
	// maxProjectConfigLayerPrefixes bounds the sorted prefix-overlap scan so a
	// hostile but still ≤1 MiB config cannot force quadratic validation CPU.
	maxProjectConfigLayerPrefixes = 4096
)

// ProjectConfigError signals a --project-config value that is missing,
// unreadable, or does not satisfy the frozen v1 schema. It maps to exit code
// 2 and is reported in the local CodeSignal document when analysis has
// already completed.
type ProjectConfigError struct {
	Message string
}

func (e *ProjectConfigError) Error() string { return e.Message }

// ProjectBackendUnavailableError signals a valid project configuration whose
// requested language has no registered project-analysis backend. It maps to
// exit code 3 and is reported in the local CodeSignal document.
type ProjectBackendUnavailableError struct {
	Message string
}

func (e *ProjectBackendUnavailableError) Error() string { return e.Message }

type projectConfig struct {
	SchemaVersion    string                   `json:"schema_version"`
	Roots            []string                 `json:"roots"`
	Layers           []projectConfigLayer     `json:"layers,omitempty"`
	ForbiddenImports []projectForbiddenImport `json:"forbidden_imports,omitempty"`
	SourceSinkPack   string                   `json:"source_sink_pack,omitempty"`
}

type projectConfigLayer struct {
	Name     string   `json:"name"`
	Prefixes []string `json:"prefixes"`
}

type projectForbiddenImport struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// LoadProjectConfig reads and validates a repository-relative config at an
// immutable Git revision. Reading through Git, rather than the worktree,
// keeps a diff report from mixing committed source facts with uncommitted
// configuration. Git stdout/stderr, wall time, document size, and JSON
// nesting are bounded at this boundary.
func LoadProjectConfig(dir, revision, repoPath string) (json.RawMessage, error) {
	if err := validateProjectConfigPath(repoPath); err != nil {
		return nil, projectConfigError(repoPath, err.Error())
	}

	data, err := runProjectConfigGit(dir, "show", revision+":"+repoPath)
	if err != nil {
		return nil, &ProjectConfigError{Message: fmt.Sprintf("coach codesignal: --project-config %q is not readable at revision %q (project_config_invalid): %s", repoPath, revision, err)}
	}
	if err := validateProjectConfigJSON(data); err != nil {
		return nil, projectConfigError(repoPath, err.Error())
	}
	return json.RawMessage(data), nil
}

// runProjectConfigGit is the Git seam used by LoadProjectConfig. Tests may
// replace it to exercise timeout and bound failures without hanging.
var runProjectConfigGit = func(dir string, args ...string) ([]byte, error) {
	return runGitBytesBounded(dir, maxProjectConfigBytes, maxProjectConfigGitStderr, projectConfigGitTimeout, args...)
}

// gitCommandContext builds the git child used by bounded reads. Tests may
// replace it to simulate hung or oversized children.
var gitCommandContext = func(ctx context.Context, dir string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
}

// runGitBytesBounded runs git with a wall-time limit and hard caps on
// collected stdout and stderr, building the child via the package's default
// gitCommandContext seam. The LimitReader stops after maxStdout+1 bytes so
// an oversized blob is detected without buffering the entire child output.
func runGitBytesBounded(dir string, maxStdout, maxStderr int64, timeout time.Duration, args ...string) ([]byte, error) {
	return runGitBytesBoundedWith(gitCommandContext, dir, maxStdout, maxStderr, timeout, args...)
}

// runGitBytesBoundedWith is the shared bounded-git-read implementation
// behind runGitBytesBounded: a wall-time limit, hard stdout/stderr caps, and
// concurrent pipe draining. buildCmd is the child-construction seam, letting
// callers vary command/environment construction (e.g. project_snapshot.go's
// sanitized-environment snapshot reads) without duplicating this I/O logic.
func runGitBytesBoundedWith(buildCmd func(ctx context.Context, dir string, args ...string) *exec.Cmd, dir string, maxStdout, maxStderr int64, timeout time.Duration, args ...string) ([]byte, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("git execution timeout must be positive")
	}
	if maxStdout < 0 || maxStderr < 0 {
		return nil, fmt.Errorf("git output bounds must be non-negative")
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := buildCmd(ctx, dir, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	// Drain stdout and stderr concurrently. Sequential reads deadlock when
	// the child fills one pipe's OS buffer while the parent is still
	// blocked reading the other.
	type pipeResult struct {
		data []byte
		err  error
	}
	stdoutCh := make(chan pipeResult, 1)
	stderrCh := make(chan pipeResult, 1)
	go func() {
		data, err := io.ReadAll(io.LimitReader(stdout, maxStdout+1))
		stdoutCh <- pipeResult{data: data, err: err}
	}()
	go func() {
		data, err := io.ReadAll(io.LimitReader(stderr, maxStderr+1))
		stderrCh <- pipeResult{data: data, err: err}
	}()
	stdoutRes := <-stdoutCh
	stderrRes := <-stderrCh
	waitErr := cmd.Wait()

	if stdoutRes.err != nil {
		return nil, stdoutRes.err
	}
	if stderrRes.err != nil {
		return nil, stderrRes.err
	}
	if int64(len(stdoutRes.data)) > maxStdout {
		return nil, fmt.Errorf("git stdout exceeded %d-byte budget", maxStdout)
	}
	if int64(len(stderrRes.data)) > maxStderr {
		return nil, fmt.Errorf("git stderr exceeded %d-byte budget", maxStderr)
	}
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("git execution timed out after %s", timeout)
	}
	if waitErr != nil {
		if len(stderrRes.data) > 0 {
			return nil, fmt.Errorf("%s: %s", waitErr, strings.TrimSpace(string(stderrRes.data)))
		}
		return nil, waitErr
	}
	return stdoutRes.data, nil
}

func projectConfigError(repoPath, reason string) error {
	return &ProjectConfigError{Message: fmt.Sprintf("coach codesignal: --project-config %q is invalid (project_config_invalid): %s", repoPath, reason)}
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

func validateProjectConfigJSON(data []byte) error {
	if int64(len(data)) > maxProjectConfigBytes {
		return fmt.Errorf("document exceeds %d-byte size budget", maxProjectConfigBytes)
	}
	if !json.Valid(data) {
		return fmt.Errorf("document is not valid JSON")
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return err
	}

	var config projectConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return fmt.Errorf("schema decode failed: %s", err)
	}
	if config.SchemaVersion != "1" {
		return fmt.Errorf("schema_version must be \"1\"")
	}
	if len(config.Roots) == 0 {
		return fmt.Errorf("roots must contain at least one repository-relative directory")
	}
	for _, root := range config.Roots {
		if err := validateProjectConfigDirectory(root); err != nil {
			return fmt.Errorf("root %q: %s", root, err)
		}
	}
	// Roots may nest (e.g. "." plus "services/payments"): multi-module Go
	// workspaces and the #220 candidate contract treat a workspace root and a
	// more specific module root as distinct configured roots. Exact duplicate
	// identities remain invalid. Layer prefixes below stay non-overlapping
	// because they partition policy membership, not discovery roots.
	if hasDuplicatePaths(config.Roots) {
		return fmt.Errorf("roots must be unique")
	}

	seenLayerNames := make(map[string]struct{}, len(config.Layers))
	var allPrefixes []string
	for _, layer := range config.Layers {
		if layer.Name == "" {
			return fmt.Errorf("layer name must be non-empty")
		}
		if _, exists := seenLayerNames[layer.Name]; exists {
			return fmt.Errorf("layer names must be unique")
		}
		seenLayerNames[layer.Name] = struct{}{}
		if len(layer.Prefixes) == 0 {
			return fmt.Errorf("layer %q must contain at least one prefix", layer.Name)
		}
		for _, prefix := range layer.Prefixes {
			if err := validateProjectConfigDirectory(prefix); err != nil {
				return fmt.Errorf("layer %q prefix %q: %s", layer.Name, prefix, err)
			}
			allPrefixes = append(allPrefixes, prefix)
		}
	}
	if len(allPrefixes) > maxProjectConfigLayerPrefixes {
		return fmt.Errorf("layer prefixes exceed budget of %d entries", maxProjectConfigLayerPrefixes)
	}
	if hasDuplicateOrOverlappingPaths(allPrefixes) {
		return fmt.Errorf("layer prefixes must be unique and non-overlapping")
	}

	seenForbidden := make(map[string]struct{}, len(config.ForbiddenImports))
	for _, forbidden := range config.ForbiddenImports {
		if forbidden.From == "" || forbidden.To == "" {
			return fmt.Errorf("forbidden_imports entries require non-empty from and to")
		}
		// A forbidden_imports entry is an explicit user claim that a layer
		// pair exists. Left unchecked, a typo'd from/to that names no
		// declared layer would validate cleanly but can never match any
		// evaluated (layerFrom, layerTo) pair, silently making that policy
		// line a permanent no-op (#211).
		if _, ok := seenLayerNames[forbidden.From]; !ok {
			return fmt.Errorf("forbidden_imports entry references undefined layer %q", forbidden.From)
		}
		if _, ok := seenLayerNames[forbidden.To]; !ok {
			return fmt.Errorf("forbidden_imports entry references undefined layer %q", forbidden.To)
		}
		key := forbidden.From + "\x00" + forbidden.To
		if _, exists := seenForbidden[key]; exists {
			return fmt.Errorf("forbidden_imports entries must be unique")
		}
		seenForbidden[key] = struct{}{}
	}
	if config.SourceSinkPack != "" && config.SourceSinkPack != "builtin-v1" {
		return fmt.Errorf("source_sink_pack must be \"builtin-v1\" when supplied")
	}
	return nil
}

func validateProjectConfigDirectory(value string) error {
	if value == "" || path.IsAbs(value) {
		return fmt.Errorf("must be a non-empty repository-relative path")
	}
	clean := path.Clean(value)
	if clean != value || clean == ".." || strings.HasPrefix(clean, "../") || strings.Contains(value, "\\") {
		return fmt.Errorf("must be a normalized repository-relative path")
	}
	return nil
}

func hasDuplicatePaths(paths []string) bool {
	seen := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		if _, exists := seen[p]; exists {
			return true
		}
		seen[p] = struct{}{}
	}
	return false
}

// hasDuplicateOrOverlappingPaths reports exact duplicates or ancestor/descendant
// path pairs. Used for layer prefixes, which must partition policy membership.
// Complexity is O(n log n) via sort + adjacent/ancestor checks rather than a
// nested all-pairs scan (F-006).
func hasDuplicateOrOverlappingPaths(paths []string) bool {
	if len(paths) < 2 {
		return false
	}
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)
	for i := 0; i < len(sorted); i++ {
		if sorted[i] == "." {
			// "." is an ancestor of every other non-empty prefix.
			if len(sorted) > 1 {
				return true
			}
			continue
		}
		if i+1 < len(sorted) {
			left, right := sorted[i], sorted[i+1]
			if left == right || strings.HasPrefix(right, left+"/") {
				return true
			}
		}
	}
	return false
}

// rejectDuplicateJSONKeys walks one JSON value and rejects duplicate object
// keys and nesting deeper than maxProjectConfigJSONDepth. encoding/json
// otherwise silently keeps the last duplicate value, which would make a
// supposedly frozen config schema depend on parser details.
func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var walk func(depth int) error
	walk = func(depth int) error {
		if depth > maxProjectConfigJSONDepth {
			return fmt.Errorf("document exceeds JSON nesting budget of %d", maxProjectConfigJSONDepth)
		}
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		switch delimiter := token.(type) {
		case json.Delim:
			switch delimiter {
			case '{':
				seen := map[string]struct{}{}
				for decoder.More() {
					keyToken, err := decoder.Token()
					if err != nil {
						return err
					}
					key := keyToken.(string)
					if _, exists := seen[key]; exists {
						return fmt.Errorf("duplicate object key %q", key)
					}
					seen[key] = struct{}{}
					if err := walk(depth + 1); err != nil {
						return err
					}
				}
				_, err = decoder.Token()
				return err
			case '[':
				for decoder.More() {
					if err := walk(depth + 1); err != nil {
						return err
					}
				}
				_, err = decoder.Token()
				return err
			}
		}
		return nil
	}
	if err := walk(1); err != nil {
		return err
	}
	if _, err := decoder.Token(); err == nil {
		return fmt.Errorf("document must contain exactly one JSON value")
	}
	return nil
}

// ResolveProjectBackend reports whether a project-analysis backend is
// registered for language. Only "go" (#211) has a registered backend today;
// every other language, including "typescript" (#214) and the empty string,
// remains unavailable until its own backend lands.
func ResolveProjectBackend(language string) error {
	if language == "go" {
		return nil
	}
	return &ProjectBackendUnavailableError{Message: fmt.Sprintf("coach codesignal: no project-analysis backend is available for language %q yet (project_backend_unavailable)", language)}
}
