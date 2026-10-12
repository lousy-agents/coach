package sourcescope

import (
	"encoding/json"
	"path/filepath"
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

// isTSConfigPathSpecifier is true for ./ ../ .\ ..\ or absolute paths only.
func isTSConfigPathSpecifier(extends string) bool {
	return strings.HasPrefix(extends, "./") ||
		strings.HasPrefix(extends, "../") ||
		strings.HasPrefix(extends, `.\`) ||
		strings.HasPrefix(extends, `..\`) ||
		filepath.IsAbs(extends)
}

func (c tsConfig) matchesExclude(path string) bool { return matchesAny(path, c.Exclude) }

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

func matchesAny(path string, patterns []string) bool {
	for _, pattern := range patterns {
		if globMatch(pattern, path) {
			return true
		}
	}
	return false
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
