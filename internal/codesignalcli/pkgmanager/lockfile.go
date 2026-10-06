package pkgmanager

import (
	"path/filepath"
)

// detectPackageManagerLockfile reports the manager kind implied by a
// recognized lockfile basename present at root. More than one distinct
// kind's lockfile committed simultaneously is reported as ambiguous rather
// than picking one arbitrarily.
func detectPackageManagerLockfile(root string) (kind string, ambiguous bool, ok bool) {
	found := map[string]bool{}
	for basename, k := range LockfileBasenames {
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

const (
	KindNPM  = "npm"
	KindPNPM = "pnpm"
	KindBun  = "bun"
	KindYarn = "yarn"
)

// LockfileBasenames maps each matrix-recognized lockfile
// basename to the manager kind it identifies (SA-280-012). Bun has two
// recognized lockfile variants: the binary bun.lockb and the newer text
// bun.lock.
var LockfileBasenames = map[string]string{
	"package-lock.json": KindNPM,
	"pnpm-lock.yaml":    KindPNPM,
	"bun.lock":          KindBun,
	"bun.lockb":         KindBun,
	"yarn.lock":         KindYarn,
}
