package codesignal

import "github.com/lousy-agents/coach/pkg/semantics"

// deepRelativeImportDepthThreshold is the minimum count of ".." segments in
// a relative import path that triggers a coupling.deep_relative_import
// signal.
const deepRelativeImportDepthThreshold = 3

const deepRelativeImportWhyItMatters = "A deep relative import (\"../../../\") tightly couples a module to another module's exact directory location, so moving or renaming either side breaks the import and makes refactors brittle."

const deepRelativeImportRecommendation = "Introduce a path alias or restructure toward a shared module boundary instead of climbing many directories with relative imports."

// deepRelativeImportDepth reports the count of ".." segments in path and
// whether path is relative (starts with "."). Segments equal to "." are
// ignored: they neither increment nor reset the count. Depth is only
// meaningful when isRelative is true.
func deepRelativeImportDepth(path string) (depth int, isRelative bool) {
	if path == "" || path[0] != '.' {
		return 0, false
	}

	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '/' {
			segment := path[start:i]
			if segment == ".." {
				depth++
			}
			start = i + 1
		}
	}
	return depth, true
}

// signalsFromImports maps one side's imports to Signals, restricted to
// languages with a relative-import concept (TypeScript/TSX). Go's import
// paths are package paths, not filesystem-relative, so this rule never
// applies to semantics.LanguageGo.
func signalsFromImports(path string, language semantics.Language, imports []semantics.ImportFeature) []Signal {
	if language != semantics.LanguageTypeScript && language != semantics.LanguageTSX {
		return nil
	}

	var signals []Signal
	for _, imp := range imports {
		depth, isRelative := deepRelativeImportDepth(imp.Path)
		if !isRelative || depth < deepRelativeImportDepthThreshold {
			continue
		}
		signals = append(signals, newDeepRelativeImportSignal(path, imp))
	}
	return signals
}

func newDeepRelativeImportSignal(path string, imp semantics.ImportFeature) Signal {
	return Signal{
		RuleID:         "coupling.deep_relative_import",
		RuleVersion:    "1",
		Kind:           "deep_relative_import",
		Category:       "coupling",
		Severity:       "medium",
		Confidence:     "medium",
		Path:           path,
		Subject:        imp.Path,
		Evidence:       imp.Path,
		Location:       imp.Location,
		WhyItMatters:   deepRelativeImportWhyItMatters,
		Recommendation: deepRelativeImportRecommendation,
		Provenance: Provenance{
			Producer: "codesignal",
		},
	}
}
