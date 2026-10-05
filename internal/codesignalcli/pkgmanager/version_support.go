package pkgmanager

import (
	"context"
	"strconv"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
	"github.com/lousy-agents/coach/internal/codesignalcli/tstoolchain"
)

func classifyProbedPackageManagerVersion(detection managerDetection) projectreadiness.Check {
	version, probed := ProbeVersion(context.Background(), detection.kind)
	if !probed || !tstoolchain.IsExactVersion(version) {
		return projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPackageManagerVersionUnverifiable, Kind: detection.kind, PinnedVersion: detection.pin}
	}
	if !packageManagerVersionSupported(detection.kind, version) {
		return projectreadiness.Check{State: projectreadiness.Fail, Code: projectreadiness.GapPackageManagerVersionUnsupported, Kind: detection.kind, FoundVersion: version, PinnedVersion: detection.pin}
	}
	return projectreadiness.Check{State: projectreadiness.Pass, Kind: detection.kind, Version: version, PinnedVersion: detection.pin}
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
	case KindNPM:
		return major == 11 && !qualified
	case KindPNPM:
		return major == 10 && !qualified
	case KindBun:
		// Bun: stable >=1.0.0 <2.0.0, excluding prerelease/canary/commit
		// builds, with no upper minor/patch bound inside 1.x.
		return major == 1 && !qualified
	default:
		return false
	}
}

// semverMajor extracts version's major component and whether it carries any
// qualifier past its bare major.minor.patch core, reusing tstoolchain.IsExactVersion's
// grammar (tstoolchain.IsExactVersion) rather than a second version
// parser. qualified is true for a semver prerelease tag ("-") and for build
// metadata ("+") alike: the frozen matrix (SA-280-012) covers bare stable
// releases only, and a manager reporting build metadata for itself is
// indistinguishable from an unverifiable commit build.
func semverMajor(version string) (major int, qualified bool, ok bool) {
	if !tstoolchain.IsExactVersion(version) {
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
