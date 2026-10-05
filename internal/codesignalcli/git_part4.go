package codesignalcli

// coverageLanguageLabel returns a stable, non-empty label for a
// CoverageGroup.Language so an extensionless file (LICENSE, Makefile;
// filepath.Ext returns "") never produces an empty, JSON-omitted label that
// renders as a blank in text output.
func coverageLanguageLabel(ext string) string {
	if ext == "" {
		return "(no extension)"
	}
	return ext
}
