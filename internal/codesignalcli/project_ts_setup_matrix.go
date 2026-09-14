package codesignalcli

import (
	"strconv"
	"strings"
)

const (
	packageManagerKindNPM  = "npm"
	packageManagerKindPNPM = "pnpm"
	packageManagerKindBun  = "bun"
	packageManagerKindYarn = "yarn"
)

// packageManagerLockfileBasenames maps each matrix-recognized lockfile
// basename to the manager kind it identifies (SA-280-012). Bun has two
// recognized lockfile variants: the binary bun.lockb and the newer text
// bun.lock.
var packageManagerLockfileBasenames = map[string]string{
	"package-lock.json": packageManagerKindNPM,
	"pnpm-lock.yaml":    packageManagerKindPNPM,
	"bun.lock":          packageManagerKindBun,
	"bun.lockb":         packageManagerKindBun,
	"yarn.lock":         packageManagerKindYarn,
}

// packageManagerVersionSupported reports whether version falls inside
// kind's frozen supported range (SA-280-012). Yarn has no supported row --
// callers must withhold it before this is ever consulted.
func packageManagerVersionSupported(kind, version string) bool {
	major, qualified, ok := semverMajor(version)
	if !ok {
		return false
	}
	switch kind {
	case packageManagerKindNPM:
		return major == 11 && !qualified
	case packageManagerKindPNPM:
		return major == 10 && !qualified
	case packageManagerKindBun:
		// Bun: stable >=1.0.0 <2.0.0, excluding prerelease/canary/commit
		// builds, with no upper minor/patch bound inside 1.x.
		return major == 1 && !qualified
	default:
		return false
	}
}

// semverMajor extracts version's major component and whether it carries any
// qualifier past its bare major.minor.patch core, reusing isExactVersion's
// grammar (project_ts_compiler_resolve.go) rather than a second version
// parser. qualified is true for a semver prerelease tag ("-") and for build
// metadata ("+") alike: the frozen matrix (SA-280-012) covers bare stable
// releases only, and a manager reporting build metadata for itself is
// indistinguishable from an unverifiable commit build.
func semverMajor(version string) (major int, qualified bool, ok bool) {
	if !isExactVersion(version) {
		return 0, false, false
	}
	core := version
	if idx := strings.IndexAny(version, "-+"); idx >= 0 {
		qualified = true
		core = version[:idx]
	}
	majorPart, _, _ := strings.Cut(core, ".")
	parsed, err := strconv.Atoi(majorPart)
	if err != nil {
		return 0, false, false
	}
	return parsed, qualified, true
}
