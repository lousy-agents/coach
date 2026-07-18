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
// belongs to, then removes files known not to ship when scope is not "all".
// Unknown files are deliberately retained so an incomplete project
// configuration cannot silently hide a finding.
func ApplySourceScope(dir, headSHA, buildTarget, scope string, files []SelectedFile) ([]SelectedFile, error) {
	classified, err := classifySourceFiles(dir, headSHA, buildTarget, scope, files)
	if err != nil {
		return nil, err
	}
	if scope == "all" {
		return classified, nil
	}

	kept := make([]SelectedFile, 0, len(classified))
	for _, file := range classified {
		if file.SourceScope != SourceScopeTestOnly && file.SourceScope != SourceScopeExcluded {
			kept = append(kept, file)
		}
	}
	return kept, nil
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

	return kept, excluded, nil
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
	// Extends names a base config this one inherits from. Only relative or
	// absolute file paths are resolved (bare npm-style specifiers, e.g.
	// "@tsconfig/node18/tsconfig.json", are not).
	Extends string   `json:"extends"`
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
	// A non-nil Files distinguishes an explicit empty "files": [] (which
	// selects no source files) from an omitted files setting.
	Files *[]string `json:"files"`
}

// loadTSConfig reads dir's tsconfig.json and, if it has an "extends" field
// naming a relative or absolute base config, resolves and applies it.
// TypeScript's "extends" semantics override, rather than merge, each of
// include/exclude/files: a field the child specifies replaces the base's
// value for that field entirely, and only an omitted field falls back to the
// base. Only a single level of extends is resolved here (no chained bases,
// no cycle detection, no npm-package-specifier resolution) — those are
// handled by later work; anything beyond a single, in-bounds, path-shaped
// extends target fails open the same as a missing tsconfig.json.
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

	base, ok := resolveExtendedTSConfig(dir, config.Extends)
	if !ok {
		// The extends target is missing, unreadable, malformed, an npm-style
		// specifier, or escapes the snapshot directory. tsconfig.json is
		// attacker-influenced input (e.g. a fork's PR diff), so any of these
		// fail open rather than reading an arbitrary path or erroring out.
		return tsConfig{}, false, nil
	}

	if config.Include == nil {
		config.Include = base.Include
	}
	if config.Exclude == nil {
		config.Exclude = base.Exclude
	}
	if config.Files == nil {
		config.Files = base.Files
	}
	return config, true, nil
}

// resolveExtendedTSConfig resolves extends relative to dir (the directory
// containing the config that references it), refusing to read anything
// outside dir. It mirrors extractTar's boundary check: after computing the
// path relative to dir, an escape shows up as ".." or a ".."-prefixed
// relative path.
func resolveExtendedTSConfig(dir, extends string) (tsConfig, bool) {
	if !isTSConfigPathSpecifier(extends) {
		return tsConfig{}, false
	}
	target := extends
	if !filepath.IsAbs(target) {
		target = filepath.Join(dir, target)
	}
	rel, err := filepath.Rel(dir, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return tsConfig{}, false
	}
	base, ok, err := readTSConfigFile(target)
	if err != nil || !ok {
		return tsConfig{}, false
	}
	return base, true
}

// isTSConfigPathSpecifier reports whether extends names a relative or
// absolute file path rather than a bare npm-style package specifier (e.g.
// "@tsconfig/node18/tsconfig.json"), which this task does not resolve.
func isTSConfigPathSpecifier(extends string) bool {
	return strings.HasPrefix(extends, "./") ||
		strings.HasPrefix(extends, "../") ||
		strings.HasPrefix(extends, `.\`) ||
		strings.HasPrefix(extends, `..\`) ||
		filepath.IsAbs(extends)
}

// readTSConfigFile reads and parses a single tsconfig.json-shaped file at
// path, without following its own "extends" field. A missing or genuinely
// malformed file reports ok=false with a nil error; only an I/O error other
// than "not exist" is surfaced as an error.
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
		// A tsconfig permits comments and trailing commas (JSONC), which
		// encoding/json cannot parse on its own. stripJSONCComments handles
		// those; any error surviving it means the file is genuinely
		// malformed for some other reason, so treat the config as
		// unestablished rather than excluding findings.
		return tsConfig{}, false, nil
	}
	return config, true, nil
}

// stripJSONCComments removes "//" line comments and "/* */" block comments
// from JSONC-flavored input, and drops trailing commas before a closing "}"
// or "]", so the result can be handed to encoding/json.Unmarshal. It tracks
// whether it is inside a double-quoted JSON string (honoring "\"" escapes)
// so a comment-like sequence inside a string value is never mistaken for a
// real comment. If a "/*" block comment is never closed before EOF, the
// input is genuinely malformed JSONC; rather than silently dropping
// everything from the unterminated "/*" onward (which could turn otherwise
// valid JSON into a truncated document that happens to parse), the original,
// unmodified data is returned so the bare "/" reliably fails
// json.Unmarshal and the caller's existing malformed-config fallback
// applies.
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
				// Ran out of input without finding the closing "*/": an
				// unterminated block comment, not a real one.
				return data
			}
			i++ // land on the closing '/'
		default:
			out.WriteByte(b)
		}
	}
	return stripTrailingCommas(out.Bytes())
}

// stripTrailingCommas removes a comma that is followed (ignoring whitespace)
// only by a closing "}" or "]", which encoding/json otherwise rejects. It is
// string-literal-aware for the same reason as stripJSONCComments.
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
				continue // drop the trailing comma
			}
		}
		out.WriteByte(b)
	}
	return out.Bytes()
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
