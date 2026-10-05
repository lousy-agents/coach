package codesignalcli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveExtendedTSConfig joins extends relative to dir, then enforces the
// snapshotRoot boundary after EvalSymlinks (gitrepo.extractTar preserves symlinks;
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
