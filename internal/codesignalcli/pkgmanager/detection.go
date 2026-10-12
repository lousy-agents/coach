package pkgmanager

import (
	"os"
)

type managerDetection struct {
	kind      string
	pin       string // package.json's packageManager version, recorded for disclosure only
	ambiguous bool
}

// detectPackageManager identifies the repository's package manager from its
// recognized metadata (SA-280-012): a package.json "packageManager" pin and a
// bare lockfile are equally good evidence of which manager a repository uses,
// and they are cross-checked against each other rather than ranked. ok is
// false only when no recognized metadata exists at all, distinct from
// detection.ambiguous (recognized metadata that disagrees with itself).
func detectPackageManager(root string) (managerDetection, bool) {
	fieldKind, fieldVersion, fieldOK := readPackageManagerField(root)
	lockKind, lockAmbiguous, lockOK := detectPackageManagerLockfile(root)
	return reconcileDetection(fieldKind, fieldVersion, fieldOK, lockKind, lockOK, lockAmbiguous)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// detectPackageManagerAcrossContexts reduces the selected roots' package
// contexts to one detection. Contexts naming different managers are reported
// as ambiguous rather than resolved to whichever root was listed first: the
// frozen rows install one manager in one working directory, so there is no
// honest way to serve two. A context with no recognized metadata contributes
// nothing and is not itself a disagreement. ok is false only when no context
// recognized anything at all.
func detectPackageManagerAcrossContexts(contexts []string) (managerDetection, bool) {
	var resolved managerDetection
	found := false
	for _, packageDir := range contexts {
		contextDetection, ok := detectPackageManager(packageDir)
		if !ok {
			continue
		}
		if contextDetection.ambiguous {
			return managerDetection{ambiguous: true}, true
		}
		if !found {
			resolved, found = contextDetection, true
			continue
		}
		if contextDetection.kind != resolved.kind {
			return managerDetection{ambiguous: true}, true
		}
		if resolved.pin == "" {
			resolved.pin = contextDetection.pin
		}
	}
	return resolved, found
}

// reconcileDetection is the single cross-check rule shared by
// the worktree and snapshot detection paths so they cannot diverge (SA-280-012).
func reconcileDetection(fieldKind, fieldPin string, fieldOK bool, lockKind string, lockOK, lockAmbiguous bool) (managerDetection, bool) {
	if lockAmbiguous || (fieldOK && lockOK && fieldKind != lockKind) {
		return managerDetection{ambiguous: true}, true
	}
	switch {
	case fieldOK:
		return managerDetection{kind: fieldKind, pin: fieldPin}, true
	case lockOK:
		return managerDetection{kind: lockKind}, true
	default:
		return managerDetection{}, false
	}
}
