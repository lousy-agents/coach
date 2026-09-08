package codesignalcli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
)

func evaluateMiseProjectOrigin(dir string) originEvaluation {
	data, err := os.ReadFile(filepath.Join(dir, "mise.toml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return noCandidateEvaluation(compilerOriginMiseProject, compilerClassUnconfigured)
		}
		return noCandidateEvaluation(compilerOriginMiseProject, compilerClassUnreadable)
	}

	versions := dedupeStrings(filterExactVersions(parseMiseToolsTypescriptVersions(string(data))))
	switch len(versions) {
	case 0:
		return noCandidateEvaluation(compilerOriginMiseProject, compilerClassUnconfigured)
	case 1:
		return originEvaluation{candidate: classifyCompilerCandidate(compilerOriginMiseProject, miseCompilerLocator(versions[0]))}
	default:
		return originEvaluation{conflict: true}
	}
}

func evaluateMiseGlobalOrigin() originEvaluation {
	version, found := detectGlobalMiseTypescriptVersion(context.Background())
	if !found || !isExactVersion(version) {
		return noCandidateEvaluation(compilerOriginMiseGlobal, compilerClassUnconfigured)
	}
	return originEvaluation{candidate: classifyCompilerCandidate(compilerOriginMiseGlobal, miseCompilerLocator(version))}
}

func miseCompilerLocator(version string) func() (string, bool) {
	return func() (string, bool) {
		return locateMiseTypescriptInstall(context.Background(), version)
	}
}

func filterExactVersions(values []string) []string {
	exact := make([]string, 0, len(values))
	for _, value := range values {
		if isExactVersion(value) {
			exact = append(exact, value)
		}
	}
	return exact
}
