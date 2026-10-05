package codesignalcli

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
