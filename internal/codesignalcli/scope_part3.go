package codesignalcli

import (
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/pkg/semantics"
)

func classifySourceFile(file gitrepo.SelectedFile, goProduction map[string]bool, buildTarget string, config tsConfig, hasTSConfig bool) string {
	switch file.Language {
	case semantics.LanguageGo:
		if goProduction[file.Path] {
			return SourceScopeProduction
		}
		if strings.HasSuffix(file.Path, "_test.go") {
			return SourceScopeTestOnly
		}
		if buildTarget == "" {
			return SourceScopeUnknown
		}
		return SourceScopeExcluded
	case semantics.LanguageTypeScript, semantics.LanguageTSX:
		if !hasTSConfig {
			return SourceScopeUnknown
		}
		if config.matchesExclude(file.Path) || !config.matchesInclude(file.Path) {
			return SourceScopeTestOnly
		}
		return SourceScopeProduction
	default:
		return SourceScopeUnknown
	}
}
