package codesignalcli

import "path/filepath"

func classifyCompilerCandidate(origin string, locate func() (string, bool)) compilerCandidate {
	candidate := compilerCandidate{origin: origin, class: compilerClassAbsent}
	location, ok := locate()
	if !ok {
		return candidate
	}
	installed, exists, unreadable := readTypescriptVersionAt(location)
	if unreadable {
		candidate.class = compilerClassUnreadable
		return candidate
	}
	if !exists {
		return candidate
	}

	candidate.version = installed
	candidate.path = location
	if !isSupportedTypescriptVersion(installed) {
		candidate.class = compilerClassUnsupported
		return candidate
	}
	nativeDir, _, nativeOK := resolveNativePackage(location, installed)
	if !nativeOK {
		candidate.class = compilerClassNativeInvalid
		return candidate
	}
	candidate.class = compilerClassEligible
	candidate.nativePath = nativeDir
	return candidate
}

func noCandidateEvaluation(origin, class string) originEvaluation {
	return originEvaluation{candidate: compilerCandidate{origin: origin, class: class}}
}

func projectCompilerLocator(manifestDir string) func() (string, bool) {
	return func() (string, bool) {
		if manifestDir == "" {
			return "", false
		}
		return filepath.Join(manifestDir, "node_modules", "typescript"), true
	}
}
