package sourcescope

import (
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func classifySourceFile(file gitrepo.SelectedFile, goProduction map[string]bool, buildTarget string, config tsConfig, hasTSConfig bool) string {
	switch file.Language {
	case semantics.LanguageGo:
		if goProduction[file.Path] {
			return Production
		}
		if strings.HasSuffix(file.Path, "_test.go") {
			return TestOnly
		}
		if buildTarget == "" {
			return Unknown
		}
		return Excluded
	case semantics.LanguageTypeScript, semantics.LanguageTSX:
		if !hasTSConfig {
			return Unknown
		}
		if config.matchesExclude(file.Path) || !config.matchesInclude(file.Path) {
			return TestOnly
		}
		return Production
	default:
		return Unknown
	}
}
