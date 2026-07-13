package codesignalcli

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lousy-agents/coach/pkg/semantics"
)

const (
	SourceScopeProduction = "production"
	SourceScopeTestOnly   = "test_only"
	SourceScopeExcluded   = "excluded"
	SourceScopeUnknown    = "unknown"
)

// ApplySourceScope labels each selected file according to the source set it
// belongs to. Production mode removes files known not to ship; unknown files
// are deliberately retained so an incomplete project configuration cannot
// silently hide a finding.
func ApplySourceScope(dir, headSHA, buildTarget, scope string, files []SelectedFile) ([]SelectedFile, error) {
	if scope == "all" {
		for i := range files {
			files[i].SourceScope = classifyFilename(files[i])
		}
		return files, nil
	}

	repositoryRoot, err := repositoryRoot(dir)
	if err != nil {
		return nil, err
	}

	// Analysis reads only committed objects from HEAD. Build source scope from
	// the same snapshot so local edits cannot affect which findings appear.
	snapshotDir, err := createSnapshot(repositoryRoot, headSHA)
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(snapshotDir)

	goProduction, err := goProductionFiles(snapshotDir, repositoryRoot, dir, buildTarget)
	if err != nil {
		return nil, err
	}
	config, hasTSConfig, err := loadTSConfig(snapshotDir)
	if err != nil {
		return nil, err
	}

	kept := make([]SelectedFile, 0, len(files))
	for _, file := range files {
		file.SourceScope = classifySourceFile(file, goProduction, buildTarget, config, hasTSConfig)
		if file.SourceScope != SourceScopeTestOnly && file.SourceScope != SourceScopeExcluded {
			kept = append(kept, file)
		}
	}
	return kept, nil
}

func classifyFilename(file SelectedFile) string {
	if strings.HasSuffix(file.Path, "_test.go") {
		return SourceScopeTestOnly
	}
	return SourceScopeUnknown
}

func classifySourceFile(file SelectedFile, goProduction map[string]bool, buildTarget string, config tsConfig, hasTSConfig bool) string {
	switch file.Language {
	case semantics.LanguageGo:
		if goProduction[file.Path] {
			return SourceScopeProduction
		}
		if buildTarget == "" {
			return SourceScopeUnknown
		}
		if strings.HasSuffix(file.Path, "_test.go") {
			return SourceScopeTestOnly
		}
		return SourceScopeExcluded
	case semantics.LanguageTypeScript, semantics.LanguageTSX:
		if !hasTSConfig {
			return SourceScopeUnknown
		}
		if config.matchesExclude(file.Path) || !config.matchesInclude(file.Path) {
			return SourceScopeTestOnly
		}
		return SourceScopeProduction
	default:
		return SourceScopeUnknown
	}
}

// goProductionFiles returns Go source files selected by the requested target.
// go list applies both dependency reachability and Go build constraints.
func goProductionFiles(snapshotDir, repositoryRoot, invocationDir, buildTarget string) (map[string]bool, error) {
	if buildTarget == "" {
		return nil, nil
	}
	target, err := snapshotBuildTarget(buildTarget, repositoryRoot, invocationDir, snapshotDir)
	if err != nil {
		return nil, err
	}
	output, err := runCommand(snapshotDir, "go", "list", "-deps", "-json", target)
	if err != nil {
		return nil, fmt.Errorf("determining Go production files for %q: %w", buildTarget, err)
	}

	var files = make(map[string]bool)
	decoder := json.NewDecoder(bytes.NewReader(output))
	for decoder.More() {
		var pkg struct {
			Dir      string
			GoFiles  []string
			CgoFiles []string
		}
		if err := decoder.Decode(&pkg); err != nil {
			return nil, fmt.Errorf("decoding go list output: %w", err)
		}
		for _, name := range append(pkg.GoFiles, pkg.CgoFiles...) {
			path, err := filepath.Rel(snapshotDir, filepath.Join(pkg.Dir, name))
			if err == nil && !strings.HasPrefix(path, ".."+string(filepath.Separator)) {
				files[filepath.ToSlash(path)] = true
			}
		}
	}
	return files, nil
}

func repositoryRoot(dir string) (string, error) {
	output, err := runGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("determining repository root: %w", err)
	}
	root := strings.TrimSpace(output)
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolving repository root: %w", err)
	}
	return resolved, nil
}

// snapshotBuildTarget preserves the meaning of relative package patterns
// supplied from a subdirectory while making them point at the HEAD snapshot.
func snapshotBuildTarget(target, repositoryRoot, invocationDir, snapshotDir string) (string, error) {
	resolvedInvocationDir, err := filepath.EvalSymlinks(invocationDir)
	if err != nil {
		return "", fmt.Errorf("resolving invocation directory: %w", err)
	}
	if filepath.IsAbs(target) {
		resolvedTarget, err := filepath.EvalSymlinks(target)
		if err != nil {
			return "", fmt.Errorf("resolving build target: %w", err)
		}
		rel, err := filepath.Rel(repositoryRoot, resolvedTarget)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("build target %q is outside the repository", target)
		}
		return filepath.Join(snapshotDir, rel), nil
	}
	if !strings.HasPrefix(target, ".") {
		return target, nil
	}
	relDir, err := filepath.Rel(repositoryRoot, resolvedInvocationDir)
	if err != nil {
		return "", fmt.Errorf("resolving build target: %w", err)
	}
	return filepath.Join(snapshotDir, relDir, target), nil
}

func createSnapshot(repositoryRoot, revision string) (string, error) {
	archive, err := runGitBytes(repositoryRoot, "archive", "--format=tar", revision)
	if err != nil {
		return "", fmt.Errorf("reading source snapshot %q: %w", revision, err)
	}
	dir, err := os.MkdirTemp("", "coach-codesignal-snapshot-*")
	if err != nil {
		return "", fmt.Errorf("creating source snapshot: %w", err)
	}
	if err := extractTar(dir, archive); err != nil {
		os.RemoveAll(dir)
		return "", fmt.Errorf("extracting source snapshot: %w", err)
	}
	return dir, nil
}

func extractTar(dir string, archive []byte) error {
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		path := filepath.Join(dir, filepath.FromSlash(header.Name))
		rel, err := filepath.Rel(dir, path)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("unsafe archive path %q", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeXGlobalHeader, tar.TypeXHeader:
			// Metadata headers are consumed by archive/tar and do not represent
			// filesystem entries in the snapshot.
			continue
		case tar.TypeDir:
			if err := os.MkdirAll(path, os.FileMode(header.Mode)); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(file, reader)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, path); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unsupported archive entry %q", header.Name)
		}
	}
}

type tsConfig struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
	// A non-nil Files distinguishes an explicit empty "files": [] (which
	// selects no source files) from an omitted files setting.
	Files *[]string `json:"files"`
}

func loadTSConfig(dir string) (tsConfig, bool, error) {
	data, err := os.ReadFile(filepath.Join(dir, "tsconfig.json"))
	if os.IsNotExist(err) {
		return tsConfig{}, false, nil
	}
	if err != nil {
		return tsConfig{}, false, fmt.Errorf("reading tsconfig.json: %w", err)
	}
	var config tsConfig
	if err := json.Unmarshal(data, &config); err != nil {
		// A tsconfig permits comments, which encoding/json cannot parse. Treat a
		// config we cannot establish as unknown rather than excluding findings.
		return tsConfig{}, false, nil
	}
	return config, true, nil
}

func (c tsConfig) matchesInclude(path string) bool {
	if c.Files != nil {
		return matchesAny(path, *c.Files)
	}
	if len(c.Include) == 0 {
		return true
	}
	return matchesAny(path, c.Include)
}

func (c tsConfig) matchesExclude(path string) bool { return matchesAny(path, c.Exclude) }

func matchesAny(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if globMatch(pattern, path) {
			return true
		}
	}
	return false
}

func globMatch(pattern, path string) bool {
	var expression strings.Builder
	expression.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					expression.WriteString("(?:.*/)?")
				} else {
					expression.WriteString(".*")
				}
			} else {
				expression.WriteString("[^/]*")
			}
		case '?':
			expression.WriteString("[^/]")
		default:
			expression.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	expression.WriteString("$")
	return regexp.MustCompile(expression.String()).MatchString(filepath.ToSlash(path))
}

func runCommand(dir, name string, args ...string) ([]byte, error) {
	command := exec.Command(name, args...)
	command.Dir = dir
	var stderr bytes.Buffer
	command.Stderr = &stderr
	output, err := command.Output()
	if err != nil && stderr.Len() > 0 {
		return nil, fmt.Errorf("%s: %s", err, strings.TrimSpace(stderr.String()))
	}
	return output, err
}
