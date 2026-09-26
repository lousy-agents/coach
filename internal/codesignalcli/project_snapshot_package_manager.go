package codesignalcli

import (
	"context"
	"path"
)

const (
	packageManagerOriginField    = "package_manager_field"
	packageManagerOriginLockfile = "lockfile"
)

// snapshotProbePackageManagerVersion is the probe seam for
// snapshotPackageManagerAtRevision. Tests may replace it to verify that the
// scan ctx is threaded through to the probe without depending on subprocess
// timing behavior.
var snapshotProbePackageManagerVersion = probePackageManagerVersion

// snapshotPackageManagerAtRevision detects the package manager from the
// committed snapshot at revision, reading package.json's packageManager
// field and recognized lockfile basenames via git-based reads (never from
// the live worktree). Only npm, pnpm, and bun are reported; yarn is omitted.
// kind is "" when no supported manager is identifiable. version is probed
// from the PATH binary when detectable, and is "" when unverifiable.
func snapshotPackageManagerAtRevision(ctx context.Context, dir, revision string, roots []string) (kind, version, origin string) {
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

	v, _ := snapshotProbePackageManagerVersion(ctx, resolved.kind)
	return resolved.kind, v, resolved.origin
}

// snapshotDetectPackageManagerAtRoot reads the package.json packageManager
// field and lockfile presence at root within revision via git, returning the
// resolved detection.
func snapshotDetectPackageManagerAtRoot(dir, revision, root string) (d snapshotDetection, ok bool) {
	pkgPath := joinRepoPath(root, "package.json")
	fieldKind, fieldOK, fieldAmbiguous := snapshotReadPackageManagerField(dir, revision, pkgPath)
	lockKind, lockOK, lockAmbiguous := snapshotDetectLockfileAtRoot(dir, revision, root)

	det, detOK := reconcilePackageManagerDetection(fieldKind, "", fieldOK, lockKind, lockOK, lockAmbiguous || fieldAmbiguous)
	if !detOK || det.ambiguous {
		return snapshotDetection{packageManagerDetection: det}, detOK
	}
	origin := packageManagerOriginLockfile
	if fieldOK {
		origin = packageManagerOriginField
	}
	return snapshotDetection{packageManagerDetection: det, origin: origin}, true
}

type snapshotDetection struct {
	packageManagerDetection
	origin string
}

// joinRepoPath joins root and name into a git-style repo-root-relative path.
// A "." root resolves to just name so git show never receives a leading "./".
func joinRepoPath(root, name string) string {
	if root == "." || root == "" {
		return name
	}
	return path.Join(root, name)
}
