package codesignalcli

import (
	"archive/tar"
	"bytes"

	"io"

	"github.com/lousy-agents/coach/pkg/semantics"

	"strings"
)

func classifySourceFile(file SelectedFile, goProduction map[string]bool, buildTarget string, config tsConfig, hasTSConfig bool) string {
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
func extractTar(dir string, archive []byte) error {
	reader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := extractTarEntry(dir, header, reader); err != nil {
			return err
		}
	}
}
