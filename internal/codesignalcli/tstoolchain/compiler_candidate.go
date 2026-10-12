package tstoolchain

import (
	"path/filepath"
)

func ClassifyCandidate(origin string, locate func() (string, bool)) candidate {
	candidate := candidate{Origin: origin, Class: ClassAbsent}
	location, ok := locate()
	if !ok {
		return candidate
	}
	installed, exists, unreadable := readTypescriptVersionAt(location)
	if unreadable {
		candidate.Class = ClassUnreadable
		return candidate
	}
	if !exists {
		return candidate
	}

	candidate.Version = installed
	candidate.Path = location
	if !IsSupportedTypescriptVersion(installed) {
		candidate.Class = ClassUnsupported
		return candidate
	}
	nativeDir, _, nativeOK := resolveNativePackage(location, installed)
	if !nativeOK {
		candidate.Class = ClassNativeInvalid
		return candidate
	}
	candidate.Class = ClassEligible
	candidate.NativePath = nativeDir
	return candidate
}

func noCandidateEvaluation(origin, class string) originEvaluation {
	return originEvaluation{candidate: candidate{Origin: origin, Class: class}}
}

func projectCompilerLocator(manifestDir string) func() (string, bool) {
	return func() (string, bool) {
		if manifestDir == "" {
			return "", false
		}
		return filepath.Join(manifestDir, "node_modules", "typescript"), true
	}
}
