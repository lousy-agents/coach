package codesignalcli

import (
	"path/filepath"
)

// detectPackageManagerAcrossContexts reduces the selected roots' package
// contexts to one detection. Contexts naming different managers are reported
// as ambiguous rather than resolved to whichever root was listed first: the
// frozen rows install one manager in one working directory, so there is no
// honest way to serve two. A context with no recognized metadata contributes
// nothing and is not itself a disagreement. ok is false only when no context
// recognized anything at all.
func detectPackageManagerAcrossContexts(contexts []string) (packageManagerDetection, bool) {
	var resolved packageManagerDetection
	found := false
	for _, packageDir := range contexts {
		detection, ok := detectPackageManager(packageDir)
		if !ok {
			continue
		}
		if detection.ambiguous {
			return packageManagerDetection{ambiguous: true}, true
		}
		if !found {
			resolved, found = detection, true
			continue
		}
		if detection.kind != resolved.kind {
			return packageManagerDetection{ambiguous: true}, true
		}
		if resolved.pin == "" {
			resolved.pin = detection.pin
		}
	}
	return resolved, found
}

// detectPackageManagerLockfile reports the manager kind implied by a
// recognized lockfile basename present at root. More than one distinct
// kind's lockfile committed simultaneously is reported as ambiguous rather
// than picking one arbitrarily.
func detectPackageManagerLockfile(root string) (kind string, ambiguous bool, ok bool) {
	found := map[string]bool{}
	for basename, k := range packageManagerLockfileBasenames {
		if fileExists(filepath.Join(root, basename)) {
			found[k] = true
		}
	}
	switch len(found) {
	case 0:
		return "", false, false
	case 1:
		for k := range found {
			return k, false, true
		}
	}
	return "", true, true
}
