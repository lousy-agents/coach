package codesignalcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

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
		if !ok || visited[basePath] {
			return tsConfig{}, false, nil
		}
		visited[basePath] = true

		config = applyTSConfigBase(config, snapshotRoot, baseDir, base)
		currentDir, extends = baseDir, base.Extends
	}
	return config, true, nil
}

// applyTSConfigBase fills whichever of config's Include/Exclude/Files the
// child left unset with base's own, rebased to base's directory.
func applyTSConfigBase(config tsConfig, snapshotRoot, baseDir string, base tsConfig) tsConfig {
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
	return config
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
			inString, escaped = advanceInsideJSONString(&out, b, escaped)
			continue
		}
		switch {
		case b == '"':
			inString = true
			out.WriteByte(b)
		case b == '/' && i+1 < len(data) && data[i+1] == '/':
			i = skipJSONCLineComment(data, i)
			if i < len(data) {
				out.WriteByte('\n')
			}
		case b == '/' && i+1 < len(data) && data[i+1] == '*':
			next, ok := skipJSONCBlockComment(data, i)
			if !ok {
				return data
			}
			i = next
		default:
			out.WriteByte(b)
		}
	}
	return stripTrailingCommas(out.Bytes())
}

// advanceInsideJSONString writes b (already known to be inside a JSON
// string literal) to out and returns the string/escape state after
// consuming it. Shared by stripJSONCComments and stripTrailingCommas so
// neither strips a comment- or comma-like byte that only appears inside a
// string value.
func advanceInsideJSONString(out *bytes.Buffer, b byte, escaped bool) (stillInString, stillEscaped bool) {
	out.WriteByte(b)
	switch {
	case escaped:
		return true, false
	case b == '\\':
		return true, true
	case b == '"':
		return false, false
	}
	return true, false
}

// skipJSONCLineComment returns the index of the '\n' terminating the "//"
// comment starting at data[i], or len(data) if it runs to EOF.
func skipJSONCLineComment(data []byte, i int) int {
	for i < len(data) && data[i] != '\n' {
		i++
	}
	return i
}

// skipJSONCBlockComment returns the index of the '/' closing the "/* */"
// comment starting at data[i:i+2]. ok is false when the comment is
// unterminated.
func skipJSONCBlockComment(data []byte, i int) (newIndex int, ok bool) {
	i += 2
	for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
		i++
	}
	if i+1 >= len(data) {
		return 0, false
	}
	return i + 1, true
}

func stripTrailingCommas(data []byte) []byte {
	var out bytes.Buffer
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		b := data[i]
		if inString {
			inString, escaped = advanceInsideJSONString(&out, b, escaped)
			continue
		}
		if b == '"' {
			inString = true
			out.WriteByte(b)
			continue
		}
		if b == ',' && trailingCommaFollowedByClose(data, i) {
			continue
		}
		out.WriteByte(b)
	}
	return out.Bytes()
}

// trailingCommaFollowedByClose reports whether the comma at data[i] is
// followed only by whitespace before a closing '}' or ']', making it a
// JSONC trailing comma to drop rather than emit.
func trailingCommaFollowedByClose(data []byte, i int) bool {
	j := i + 1
	for j < len(data) && (data[j] == ' ' || data[j] == '\t' || data[j] == '\n' || data[j] == '\r') {
		j++
	}
	return j < len(data) && (data[j] == '}' || data[j] == ']')
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
