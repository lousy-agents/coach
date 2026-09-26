package codesignalcli

import (
	"encoding/json"
	"strings"
)

// snapshotReadPackageManagerField reads package.json at repoPath within
// revision and extracts the Corepack-style "packageManager" field name.
// ambiguous is true when a git operational error prevents determining whether
// the field is present, distinct from a clean absent file (ok=false, ambiguous=false).
func snapshotReadPackageManagerField(dir, revision, repoPath string) (kind string, ok bool, ambiguous bool) {
	exists, err := fileExistsAtRevision(dir, revision, repoPath)
	if err != nil {
		return "", false, true
	}
	if !exists {
		return "", false, false
	}
	data, err := runProjectConfigGit(dir, "show", revision+":"+repoPath)
	if err != nil {
		return "", false, true
	}
	var manifest struct {
		PackageManager string `json:"packageManager"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.PackageManager == "" {
		return "", false, false
	}
	name, rest, found := strings.Cut(manifest.PackageManager, "@")
	if !found || rest == "" {
		return "", false, false
	}
	switch name {
	case packageManagerKindNPM, packageManagerKindPNPM, packageManagerKindBun, packageManagerKindYarn:
		return name, true, false
	default:
		return "", false, false
	}
}

// snapshotDetectLockfileAtRoot checks recognized lockfile basenames at root
// within revision. Multiple distinct kinds are reported as ambiguous. A git
// error on any basename is also treated as ambiguous rather than continuing
// the scan: a transient failure must not silently change which manager is
// detected.
func snapshotDetectLockfileAtRoot(dir, revision, root string) (kind string, ok bool, ambiguous bool) {
	found := map[string]bool{}
	for basename, k := range packageManagerLockfileBasenames {
		repoPath := joinRepoPath(root, basename)
		exists, err := fileExistsAtRevision(dir, revision, repoPath)
		if err != nil {
			return "", false, true
		}
		if !exists {
			continue
		}
		found[k] = true
	}
	switch len(found) {
	case 0:
		return "", false, false
	case 1:
		for k := range found {
			return k, true, false
		}
	}
	return "", false, true
}
