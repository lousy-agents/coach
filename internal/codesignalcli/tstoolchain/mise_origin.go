package tstoolchain

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
			return noCandidateEvaluation(OriginMiseProject, ClassUnconfigured)
		}
		return noCandidateEvaluation(OriginMiseProject, ClassUnreadable)
	}

	versions := DedupeStrings(filterExactVersions(ParseMiseToolsTypescriptVersions(string(data))))
	switch len(versions) {
	case 0:
		return noCandidateEvaluation(OriginMiseProject, ClassUnconfigured)
	case 1:
		return originEvaluation{candidate: ClassifyCandidate(OriginMiseProject, miseCompilerLocator(versions[0]))}
	default:
		return originEvaluation{conflict: true}
	}
}

func evaluateMiseGlobalOrigin() originEvaluation {
	version, found := DetectGlobalMiseTypescriptVersion(context.Background())
	if !found || !IsExactVersion(version) {
		return noCandidateEvaluation(OriginMiseGlobal, ClassUnconfigured)
	}
	return originEvaluation{candidate: ClassifyCandidate(OriginMiseGlobal, miseCompilerLocator(version))}
}

func miseCompilerLocator(version string) func() (string, bool) {
	return func() (string, bool) {
		return LocateMiseTypescriptInstall(context.Background(), version)
	}
}

func filterExactVersions(values []string) []string {
	exact := make([]string, 0, len(values))
	for _, value := range values {
		if IsExactVersion(value) {
			exact = append(exact, value)
		}
	}
	return exact
}
