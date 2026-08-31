package codesignalcli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Project-config boundary budgets. Config is repository-controlled input and
// must fail closed before unbounded memory/CPU or a hung git child can stall
// the CLI.
const (
	maxProjectConfigBytes     = 1 << 20 // 1 MiB
	maxProjectConfigJSONDepth = 32
	maxProjectConfigGitStderr = 64 << 10 // 64 KiB
	projectConfigGitTimeout   = 30 * time.Second
	// maxProjectConfigLayerPrefixes bounds the sorted prefix-overlap scan so a
	// hostile but still ≤1 MiB config cannot force quadratic validation CPU.
	maxProjectConfigLayerPrefixes = 4096
	// maxProjectConfigRoots bounds the declared roots list. Unlike layer
	// prefixes (a pure in-process string-comparison budget), each declared
	// root can drive up to three git child-process spawns in
	// checkProjectShape's non-root package.json probe
	// (project_readiness.go), so this budget must stay small enough that
	// even the worst case (no package.json under any root) completes in a
	// few seconds rather than fanning out into tens of thousands of git
	// invocations from a config that is still well under
	// maxProjectConfigBytes.
	maxProjectConfigRoots = 256
)

// ProjectConfigError signals a --project-config value that is missing,
// unreadable, or does not satisfy the frozen v1 schema. It maps to exit code
// 2 and is reported as a single stderr message; no report is written to
// stdout.
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

// gitOperationalBoundErrorKind distinguishes which of runGitBytesBoundedWith's
// own bounds tripped, so a caller can classify a size-budget failure
// (customer-controlled content) differently from a timeout or stderr-budget
// failure (a resource/environment condition, not content the config author
// can shrink by hand).
type gitOperationalBoundErrorKind int

const (
	gitOperationalBoundTimeout gitOperationalBoundErrorKind = iota
	gitOperationalBoundStdout
	gitOperationalBoundStderr
)

// gitOperationalBoundError marks a runGitBytesBoundedWith failure that comes
// from our own timeout/output-budget enforcement rather than from git's
// stderr. Its message is already complete and safe to surface verbatim to a
// --project-config user: unlike a git failure, it never embeds raw git
// stderr.
type gitOperationalBoundError struct {
	message string
	kind    gitOperationalBoundErrorKind
}

func (e *gitOperationalBoundError) Error() string { return e.message }

type projectConfig struct {
	SchemaVersion    string                   `json:"schema_version"`
	Roots            []string                 `json:"roots"`
	Layers           []projectConfigLayer     `json:"layers,omitempty"`
	ForbiddenImports []projectForbiddenImport `json:"forbidden_imports,omitempty"`
	SourceSinkPack   string                   `json:"source_sink_pack,omitempty"`
	RequiredLayer    string                   `json:"required_layer,omitempty"`
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
		return &ProjectConfigError{Message: fmt.Sprintf("coach codesignal: --project-config %q is not readable at revision %q (project_config_invalid): %s", repoPath, revision, boundErr.Error())}
	}
	if configExistsInWorktree(dir, repoPath) {
		return &ProjectConfigError{Message: fmt.Sprintf("coach codesignal: --project-config %q exists in the worktree but is not committed at revision %q (project_config_invalid): commit the file so it is readable at the analyzed revision", repoPath, revision)}
	}
	return &ProjectConfigError{Message: fmt.Sprintf("coach codesignal: --project-config %q was not found at revision %q (project_config_invalid)", repoPath, revision)}
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
		return nil, &gitOperationalBoundError{kind: gitOperationalBoundStdout, message: fmt.Sprintf("git stdout exceeded %d-byte budget", maxStdout)}
	}
	if int64(len(stderrRes.data)) > maxStderr {
		return nil, &gitOperationalBoundError{kind: gitOperationalBoundStderr, message: fmt.Sprintf("git stderr exceeded %d-byte budget", maxStderr)}
	}
	if ctx.Err() == context.DeadlineExceeded {
		return nil, &gitOperationalBoundError{kind: gitOperationalBoundTimeout, message: fmt.Sprintf("git execution timed out after %s", timeout)}
	}
	if waitErr != nil {
		if len(stderrRes.data) > 0 {
			return nil, fmt.Errorf("%s: %s", waitErr, strings.TrimSpace(string(stderrRes.data)))
		}
		return nil, waitErr
	}
	return stdoutRes.data, nil
}

func projectConfigError(repoPath, revision, reason string) error {
	return &ProjectConfigError{Message: fmt.Sprintf("coach codesignal: --project-config %q is invalid at revision %q (project_config_invalid): %s", repoPath, revision, reason)}
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
	_, err := parseProjectConfig(data)
	return err
}

// parseProjectConfig performs validateProjectConfigJSON's full decode and
// schema validation, additionally returning the decoded projectConfig. It
// exists so a caller that needs the decoded value (checkPolicy, via
// loadProjectConfigForReadiness) never has to run a second, redundant decode
// of bytes validateProjectConfigJSON already accepted.
func parseProjectConfig(data []byte) (projectConfig, error) {
	config, err := decodeProjectConfig(data)
	if err != nil {
		return projectConfig{}, err
	}
	if err := validateProjectConfigRoots(config.Roots); err != nil {
		return projectConfig{}, err
	}
	seenLayerNames, err := validateProjectConfigLayers(config.Layers)
	if err != nil {
		return projectConfig{}, err
	}
	if err := validateProjectConfigForbiddenImports(config.ForbiddenImports, seenLayerNames); err != nil {
		return projectConfig{}, err
	}
	if err := validateProjectConfigCrossFields(config, seenLayerNames); err != nil {
		return projectConfig{}, err
	}
	return config, nil
}

func decodeProjectConfig(data []byte) (projectConfig, error) {
	if int64(len(data)) > maxProjectConfigBytes {
		return projectConfig{}, fmt.Errorf("document exceeds %d-byte size budget", maxProjectConfigBytes)
	}
	if !json.Valid(data) {
		return projectConfig{}, fmt.Errorf("document is not valid JSON")
	}
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return projectConfig{}, err
	}

	var config projectConfig
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return projectConfig{}, fmt.Errorf("schema decode failed: %s", err)
	}
	if config.SchemaVersion != "1" {
		return projectConfig{}, fmt.Errorf("schema_version must be \"1\"")
	}
	return config, nil
}

func validateProjectConfigRoots(roots []string) error {
	if len(roots) == 0 {
		return fmt.Errorf("roots must contain at least one repository-relative directory")
	}
	if len(roots) > maxProjectConfigRoots {
		return fmt.Errorf("roots exceed budget of %d entries", maxProjectConfigRoots)
	}
	for _, root := range roots {
		if err := validateProjectConfigDirectory(root); err != nil {
			return fmt.Errorf("root %q: %s", root, err)
		}
	}
	// Roots may nest (e.g. "." plus "services/payments"): a multi-module Go
	// workspace treats a workspace root and a more specific module root as
	// distinct configured roots. Exact duplicate
	// identities remain invalid. Layer prefixes below stay non-overlapping
	// because they partition policy membership, not discovery roots.
	if hasDuplicatePaths(roots) {
		return fmt.Errorf("roots must be unique")
	}
	return nil
}

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

func validateProjectConfigForbiddenImports(forbiddenImports []projectForbiddenImport, seenLayerNames map[string]struct{}) error {
	seenForbidden := make(map[string]struct{}, len(forbiddenImports))
	for _, forbidden := range forbiddenImports {
		if forbidden.From == "" || forbidden.To == "" {
			return fmt.Errorf("forbidden_imports entries require non-empty from and to")
		}
		// A forbidden_imports entry is an explicit user claim that a layer
		// pair exists. Left unchecked, a typo'd from/to that names no
		// declared layer would validate cleanly but can never match any
		// evaluated (layerFrom, layerTo) pair, silently making that policy
		// line a permanent no-op.
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
	return nil
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

// loadProjectConfigForReadiness reads and validates repoPath at revision
// like LoadProjectConfig, but keeps a git-read failure distinct from a
// content/schema rejection instead of collapsing both into
// *ProjectConfigError: checkPolicy must report the former as an
// *OperationalError (exit 1, fail closed) and only the latter as the
// policy_invalid gap. It returns the decoded projectConfig directly so
// checkPolicy needs no second decode of the same bytes.
//
// A stdout-size-budget failure is deliberately classified as content
// rejection (policy_invalid, exit 0) rather than operational (exit 1),
// diverging from LoadProjectConfig's projectConfigGitError, which reports
// every *gitOperationalBoundError -- including this same size-budget case --
// as project_config_invalid at exit 2. The committed policy file's size is
// something its author controls and can fix, unlike a corrupt object store
// or a timed-out git process, so --check-project's read-only, actionable-gap
// contract treats it as a gap rather than an environment failure. A timeout
// or stderr-budget failure still reports *OperationalError: those indicate a
// resource/environment condition, not a defect in the file's content.
func loadProjectConfigForReadiness(dir, revision, repoPath string) (projectConfig, error) {
	if err := validateProjectConfigPath(repoPath); err != nil {
		return projectConfig{}, projectConfigError(repoPath, revision, err.Error())
	}

	data, err := runProjectConfigGit(dir, "show", revision+":"+repoPath)
	if err != nil {
		var boundErr *gitOperationalBoundError
		if errors.As(err, &boundErr) && boundErr.kind == gitOperationalBoundStdout {
			return projectConfig{}, projectConfigError(repoPath, revision, fmt.Sprintf("committed content exceeds the %d-byte size budget: %s", maxProjectConfigBytes, err))
		}
		return projectConfig{}, &OperationalError{Message: fmt.Sprintf("coach codesignal --check-project: --project-config %q could not be read at revision %q: %s", repoPath, revision, err)}
	}

	config, err := parseProjectConfig(data)
	if err != nil {
		return projectConfig{}, projectConfigError(repoPath, revision, err.Error())
	}
	return config, nil
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
// nested all-pairs scan.
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
// registered for language. "go" and "typescript" both have registered
// backends today; every other language, including the empty string, remains
// unavailable until its own backend lands.
func ResolveProjectBackend(language string) error {
	if language == "go" || language == "typescript" {
		return nil
	}
	return &ProjectBackendUnavailableError{Message: fmt.Sprintf("coach codesignal: no project-analysis backend is available for language %q yet (project_backend_unavailable)", language)}
}
