package codesignalcli

import (
	"path/filepath"
	"regexp"
	"strings"
)

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

func globMatch(pattern, path string) bool {
	var expression strings.Builder
	expression.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			i = writeGlobStar(&expression, pattern, i)
		case '?':
			expression.WriteString("[^/]")
		default:
			expression.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	expression.WriteString("$")
	return regexp.MustCompile(expression.String()).MatchString(filepath.ToSlash(path))
}

func writeGlobStar(expression *strings.Builder, pattern string, i int) int {
	if i+1 < len(pattern) && pattern[i+1] == '*' {
		i++
		if i+1 < len(pattern) && pattern[i+1] == '/' {
			i++
			expression.WriteString("(?:.*/)?")
			return i
		}
		expression.WriteString(".*")
		return i
	}
	expression.WriteString("[^/]*")
	return i
}
