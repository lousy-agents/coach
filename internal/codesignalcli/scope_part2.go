package codesignalcli

import (
	"bytes"
	"encoding/json"
	"fmt"

	"path/filepath"

	"strings"
)

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
