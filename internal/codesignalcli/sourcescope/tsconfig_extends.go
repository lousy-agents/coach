package sourcescope

import (
	"os"
	"path/filepath"
	"strings"
)

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

// resolveExtendedTSConfig joins extends relative to dir, then enforces the
// snapshotRoot boundary after EvalSymlinks (gitrepo.ExtractRevision preserves symlinks;
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
