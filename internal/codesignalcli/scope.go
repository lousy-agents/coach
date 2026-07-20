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
	"sort"
	"strings"

	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/semantics"
)

const (
	SourceScopeProduction = "production"
	SourceScopeTestOnly   = "test_only"
	SourceScopeExcluded   = "excluded"
	SourceScopeUnknown    = "unknown"
)

// ApplySourceScope labels each selected file according to the source set it
// belongs to, then splits out files known not to ship when scope is not
// "all" into excluded, grouped by (SourceScope reason, Language) pair, so
// the diff flow can record what was left out and why. Unknown files are
// deliberately retained in kept so an incomplete project configuration
// cannot silently hide a finding.
func ApplySourceScope(dir, headSHA, buildTarget, scope string, files []SelectedFile) (kept []SelectedFile, excluded []codesignal.CoverageGroup, err error) {
	classified, err := classifySourceFiles(dir, headSHA, buildTarget, scope, files)
	if err != nil {
		return nil, nil, err
	}
	if scope == "all" {
		return classified, nil, nil
	}

	kept, excluded = tallyClassified(classified)
	return kept, excluded, nil
}

// ApplyBaselineSourceScope labels each selected file according to the
// source set it belongs to, same as ApplySourceScope, but instead of
// silently dropping test_only/excluded files it tallies them into excluded,
// grouped by (SourceScope reason, Language) pair, so a Repository Baseline
// report can record what was left out and why. When scope is "all",
// nothing is excluded, matching ApplySourceScope's "all" semantics.
func ApplyBaselineSourceScope(dir, revisionSHA, buildTarget, scope string, files []SelectedFile) (kept []SelectedFile, excluded []codesignal.CoverageGroup, err error) {
	classified, err := classifySourceFiles(dir, revisionSHA, buildTarget, scope, files)
	if err != nil {
		return nil, nil, err
	}
	if scope == "all" {
		return classified, nil, nil
	}

	kept, excluded = tallyClassified(classified)
	return kept, excluded, nil
}

// tallyClassified splits classified (files already labeled by
// classifySourceFiles) into files that ship (kept) and files that don't
// (excluded), grouped by (SourceScope reason, Language) pair. It is shared
// by ApplySourceScope and ApplyBaselineSourceScope, whose only difference is
// what they do with the two results.
func tallyClassified(classified []SelectedFile) (kept []SelectedFile, excluded []codesignal.CoverageGroup) {
	type groupKey struct{ reason, language string }
	counts := make(map[groupKey]int)

	kept = make([]SelectedFile, 0, len(classified))
	for _, file := range classified {
		if file.SourceScope == SourceScopeTestOnly || file.SourceScope == SourceScopeExcluded {
			counts[groupKey{reason: file.SourceScope, language: string(file.Language)}]++
			continue
		}
		kept = append(kept, file)
	}

	keys := make([]groupKey, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].reason != keys[j].reason {
			return keys[i].reason < keys[j].reason
		}
		return keys[i].language < keys[j].language
	})

	for _, key := range keys {
		excluded = append(excluded, codesignal.CoverageGroup{
			Reason:   key.reason,
			Language: key.language,
			Count:    counts[key],
		})
	}

	return kept, excluded
}

// classifySourceFiles labels each selected file's SourceScope without
// filtering any of them out, so ApplySourceScope and
// ApplyBaselineSourceScope can share the classification logic while
// applying different policies for what happens to test_only/excluded files.
func classifySourceFiles(dir, headSHA, buildTarget, scope string, files []SelectedFile) ([]SelectedFile, error) {
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

	// Analysis reads only committed objects from headSHA. Build source scope
	// from the same snapshot so local edits cannot affect which findings
	// appear.
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

	classified := make([]SelectedFile, len(files))
	for i, file := range files {
		file.SourceScope = classifySourceFile(file, goProduction, buildTarget, config, hasTSConfig)
		classified[i] = file
	}
	return classified, nil
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
		if strings.HasSuffix(file.Path, "_test.go") {
			return SourceScopeTestOnly
		}
		if buildTarget == "" {
			return SourceScopeUnknown
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
	// Path-only; npm package extends are not resolved. Non-string (e.g. TS 5
	// multi-base arrays) is ignored so the rest of the file still parses.
	Extends string   `json:"extends"`
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
	// Non-nil distinguishes explicit "files": [] (selects nothing) from omitted.
	Files *[]string `json:"files"`
}

// UnmarshalJSON accepts only string extends; other shapes leave Extends empty
// without failing the whole config.
func (c *tsConfig) UnmarshalJSON(data []byte) error {
	type tsConfigAlias tsConfig
	var aux struct {
		Extends json.RawMessage `json:"extends"`
		tsConfigAlias
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	*c = tsConfig(aux.tsConfigAlias)
	if len(aux.Extends) > 0 {
		var extends string
		if err := json.Unmarshal(aux.Extends, &extends); err == nil {
			c.Extends = extends
		}
	}
	return nil
}

// loadTSConfig resolves path-shaped extends chains. Child fields override
// base fields (no merge). Inherited patterns are rebased to the declaring
// base's directory. Cycles, escapes, npm extends, and I/O failures fail open
// (ok=false) — tsconfig is attacker-influenced (e.g. fork PR input).
func loadTSConfig(dir string) (tsConfig, bool, error) {
	config, ok, err := readTSConfigFile(filepath.Join(dir, "tsconfig.json"))
	if err != nil {
		return tsConfig{}, false, err
	}
	if !ok {
		return tsConfig{}, false, nil
	}
	if config.Extends == "" {
		return config, true, nil
	}

	visited := map[string]bool{filepath.Clean(filepath.Join(dir, "tsconfig.json")): true}

	// EvalSymlinks so rebase math uses the same path space as baseDir.
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return tsConfig{}, false, nil
	}

	snapshotRoot, currentDir, extends := resolvedDir, dir, config.Extends
	for extends != "" {
		base, baseDir, basePath, ok := resolveExtendedTSConfig(snapshotRoot, currentDir, extends)
		if !ok {
			return tsConfig{}, false, nil
		}
		if visited[basePath] {
			return tsConfig{}, false, nil
		}
		visited[basePath] = true

		if config.Include == nil {
			config.Include = rebaseTSConfigPatterns(snapshotRoot, baseDir, base.Include)
		}
		if config.Exclude == nil {
			config.Exclude = rebaseTSConfigPatterns(snapshotRoot, baseDir, base.Exclude)
		}
		if config.Files == nil && base.Files != nil {
			rebased := rebaseTSConfigPatterns(snapshotRoot, baseDir, *base.Files)
			config.Files = &rebased
		}

		currentDir, extends = baseDir, base.Extends
	}
	return config, true, nil
}

// resolveExtendedTSConfig joins extends relative to dir, then enforces the
// snapshotRoot boundary after EvalSymlinks (extractTar preserves symlinks;
// a lexical-only check would read through an in-bounds symlink to a host
// path). Boundary is snapshotRoot, not the current hop's directory.
func resolveExtendedTSConfig(snapshotRoot, dir, extends string) (config tsConfig, baseDir, basePath string, ok bool) {
	if !isTSConfigPathSpecifier(extends) {
		return tsConfig{}, "", "", false
	}
	target := extends
	if !filepath.IsAbs(target) {
		target = filepath.Join(dir, target)
	}
	target = resolveTSConfigExtendsTarget(target)

	resolvedRoot, err := filepath.EvalSymlinks(snapshotRoot)
	if err != nil {
		return tsConfig{}, "", "", false
	}
	resolvedTarget, err := filepath.EvalSymlinks(target)
	if err != nil {
		return tsConfig{}, "", "", false
	}
	rel, err := filepath.Rel(resolvedRoot, resolvedTarget)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return tsConfig{}, "", "", false
	}
	base, found, err := readTSConfigFile(resolvedTarget)
	if err != nil || !found {
		return tsConfig{}, "", "", false
	}
	return base, filepath.Dir(resolvedTarget), filepath.Clean(resolvedTarget), true
}

// resolveTSConfigExtendsTarget retries with ".json" when the literal path is missing.
func resolveTSConfigExtendsTarget(target string) string {
	if strings.HasSuffix(target, ".json") {
		return target
	}
	if info, err := os.Stat(target); err == nil && !info.IsDir() {
		return target
	}
	return target + ".json"
}

// rebaseTSConfigPatterns prefixes patterns with baseDir relative to snapshotRoot
// (TS resolves them against the declaring config's directory). Nil stays nil.
func rebaseTSConfigPatterns(snapshotRoot, baseDir string, patterns []string) []string {
	if patterns == nil {
		return nil
	}
	relBaseDir, err := filepath.Rel(snapshotRoot, baseDir)
	if err != nil {
		return patterns
	}
	rebased := make([]string, len(patterns))
	for i, pattern := range patterns {
		rebased[i] = filepath.ToSlash(filepath.Join(relBaseDir, pattern))
	}
	return rebased
}

// isTSConfigPathSpecifier is true for ./ ../ .\ ..\ or absolute paths only.
func isTSConfigPathSpecifier(extends string) bool {
	return strings.HasPrefix(extends, "./") ||
		strings.HasPrefix(extends, "../") ||
		strings.HasPrefix(extends, `.\`) ||
		strings.HasPrefix(extends, `..\`) ||
		filepath.IsAbs(extends)
}

// readTSConfigFile does not follow extends. Missing/malformed => ok=false;
// other I/O errors are returned.
func readTSConfigFile(path string) (tsConfig, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return tsConfig{}, false, nil
	}
	if err != nil {
		return tsConfig{}, false, fmt.Errorf("reading %s: %w", path, err)
	}
	var config tsConfig
	if err := json.Unmarshal(stripJSONCComments(data), &config); err != nil {
		return tsConfig{}, false, nil
	}
	return config, true, nil
}

// stripJSONCComments strips // and /* */ outside strings, then trailing commas.
// Unterminated /* returns the original bytes so Unmarshal fails closed.
func stripJSONCComments(data []byte) []byte {
	var out bytes.Buffer
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		b := data[i]
		if inString {
			out.WriteByte(b)
			switch {
			case escaped:
				escaped = false
			case b == '\\':
				escaped = true
			case b == '"':
				inString = false
			}
			continue
		}
		switch {
		case b == '"':
			inString = true
			out.WriteByte(b)
		case b == '/' && i+1 < len(data) && data[i+1] == '/':
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i < len(data) {
				out.WriteByte('\n')
			}
		case b == '/' && i+1 < len(data) && data[i+1] == '*':
			i += 2
			for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
				i++
			}
			if i+1 >= len(data) {
				return data
			}
			i++
		default:
			out.WriteByte(b)
		}
	}
	return stripTrailingCommas(out.Bytes())
}

func stripTrailingCommas(data []byte) []byte {
	var out bytes.Buffer
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		b := data[i]
		if inString {
			out.WriteByte(b)
			switch {
			case escaped:
				escaped = false
			case b == '\\':
				escaped = true
			case b == '"':
				inString = false
			}
			continue
		}
		if b == '"' {
			inString = true
			out.WriteByte(b)
			continue
		}
		if b == ',' {
			j := i + 1
			for j < len(data) && (data[j] == ' ' || data[j] == '\t' || data[j] == '\n' || data[j] == '\r') {
				j++
			}
			if j < len(data) && (data[j] == '}' || data[j] == ']') {
				continue
			}
		}
		out.WriteByte(b)
	}
	return out.Bytes()
}

// matchesInclude is the union of files and include (TS semantics). Match-all
// only when both are absent; explicit empty files selects nothing.
func (c tsConfig) matchesInclude(path string) bool {
	if c.Files != nil && matchesAny(path, *c.Files) {
		return true
	}
	if len(c.Include) > 0 {
		return matchesAny(path, c.Include)
	}
	return c.Files == nil
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
