package sourcescope

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
