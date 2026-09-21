package codesignalcli

import (
	"context"
	"encoding/json"
	"path"
	"strings"
)

const (
	packageManagerOriginField    = "package_manager_field"
	packageManagerOriginLockfile = "lockfile"
)

// snapshotPackageManagerAtRevision detects the package manager from the
// committed snapshot at revision, reading package.json's packageManager
// field and recognized lockfile basenames via git-based reads (never from
// the live worktree). Only npm, pnpm, and bun are reported; yarn is omitted.
// kind is "" when no supported manager is identifiable. version is probed
// from the PATH binary when detectable, and is "" when unverifiable.
func snapshotPackageManagerAtRevision(dir, revision string, roots []string) (kind, version, origin string) {
	if len(roots) == 0 {
		roots = []string{"."}
	}

	var resolved snapshotDetection
	found := false

	for _, root := range roots {
		d, ok := snapshotDetectPackageManagerAtRoot(dir, revision, root)
		if !ok {
			continue
		}
		if d.ambiguous {
			return "", "", ""
		}
		if !found {
			resolved, found = d, true
			continue
		}
		if d.kind != resolved.kind {
			return "", "", ""
		}
	}

	if !found || resolved.kind == "" {
		return "", "", ""
	}

	switch resolved.kind {
	case packageManagerKindNPM, packageManagerKindPNPM, packageManagerKindBun:
	default:
		return "", "", ""
	}

	v, _ := probePackageManagerVersion(context.Background(), resolved.kind)
	return resolved.kind, v, resolved.origin
}

// snapshotDetectPackageManagerAtRoot reads the package.json packageManager
// field and lockfile presence at root within revision via git, returning the
// resolved detection.
func snapshotDetectPackageManagerAtRoot(dir, revision, root string) (d snapshotDetection, ok bool) {
	pkgPath := joinRepoPath(root, "package.json")
	fieldKind, fieldOK := snapshotReadPackageManagerField(dir, revision, pkgPath)
	lockKind, lockOK, lockAmbiguous := snapshotDetectLockfileAtRoot(dir, revision, root)

	if lockAmbiguous {
		return snapshotDetection{packageManagerDetection: packageManagerDetection{ambiguous: true}}, true
	}
	switch {
	case fieldOK && lockOK && fieldKind != lockKind:
		return snapshotDetection{packageManagerDetection: packageManagerDetection{ambiguous: true}}, true
	case fieldOK:
		return snapshotDetection{
			packageManagerDetection: packageManagerDetection{kind: fieldKind},
			origin:                  packageManagerOriginField,
		}, true
	case lockOK:
		return snapshotDetection{
			packageManagerDetection: packageManagerDetection{kind: lockKind},
			origin:                  packageManagerOriginLockfile,
		}, true
	default:
		return snapshotDetection{}, false
	}
}

type snapshotDetection struct {
	packageManagerDetection
	origin string
}

// snapshotReadPackageManagerField reads package.json at repoPath within
// revision and extracts the Corepack-style "packageManager" field name.
func snapshotReadPackageManagerField(dir, revision, repoPath string) (kind string, ok bool) {
	data, err := runProjectConfigGit(dir, "show", revision+":"+repoPath)
	if err != nil {
		return "", false
	}
	var manifest struct {
		PackageManager string `json:"packageManager"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil || manifest.PackageManager == "" {
		return "", false
	}
	name, rest, found := strings.Cut(manifest.PackageManager, "@")
	if !found || rest == "" {
		return "", false
	}
	switch name {
	case packageManagerKindNPM, packageManagerKindPNPM, packageManagerKindBun, packageManagerKindYarn:
		return name, true
	default:
		return "", false
	}
}

// snapshotDetectLockfileAtRoot checks recognized lockfile basenames at root
// within revision. Multiple distinct kinds are reported as ambiguous.
func snapshotDetectLockfileAtRoot(dir, revision, root string) (kind string, ok bool, ambiguous bool) {
	found := map[string]bool{}
	for basename, k := range packageManagerLockfileBasenames {
		repoPath := joinRepoPath(root, basename)
		exists, err := fileExistsAtRevision(dir, revision, repoPath)
		if err != nil || !exists {
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

// joinRepoPath joins root and name into a git-style repo-root-relative path.
// A "." root resolves to just name so git show never receives a leading "./".
func joinRepoPath(root, name string) string {
	if root == "." || root == "" {
		return name
	}
	return path.Join(root, name)
}
