package codesignalcli

import (
	"fmt"
	"strings"
)

// miseToolSupportedCalverYear is the calver year of the mise release this
// codebase is tested against, per the frozen mise-origin row: "supported
// mise 2026.x (calver year of the tested release)". mise releases use
// calver (YYYY.M.P); the tested release is whatever mise.toml already pins
// via min_version, not a value chosen during coding. This constant must be
// updated together with mise.toml's min_version year -- see
// TestMiseToolSupportedCalverYearTracksMiseToml.
const miseToolSupportedCalverYear = "2026"

const miseNpmTypescriptTool = "npm:typescript"

// miseInstallCommandTemplate and miseWhereCommand are the frozen mise-origin
// row's exact command sequence: install the exact resolved version, then
// locate it with `mise where` (miseInstallToolSpec and
// locateMiseTypescriptInstall use this same sequence).
const miseInstallCommandTemplate = "mise install " + miseNpmTypescriptTool + "@%s"

const miseWhereCommand = "mise where"

// miseBackingNpmSuppressionFlag is the same suppression flag the frozen npm
// adapter row uses: the mise-origin row installs npm:typescript via npm's
// own resolution under the hood, so its backing npm invocation must satisfy
// this same suppression, proven with the same sentinel fixture, rather than
// a mise-specific mechanism invented separately.
const miseBackingNpmSuppressionFlag = "--ignore-scripts"

// miseInstallCommand renders the frozen row's install command for an exact
// resolved version. The frozen row names exactly one version per
// installation -- never a secondary or fallback version -- so this takes a
// single version string, not a collection.
func miseInstallCommand(version string) string {
	return fmt.Sprintf(miseInstallCommandTemplate, version)
}

// isMiseToolVersionInRow reports whether a detected `mise` tool version
// falls within the frozen mise-origin row's supported release: the same
// calver year as miseToolSupportedCalverYear. version is an already-detected
// version token (e.g. "2026.9.5"); extracting that token out of `mise
// --version`'s full output, and any config-hazard detection, is a later
// task's concern, not this function's. See miseToolReadiness for how this
// differs from classifying which TypeScript version mise installed.
func isMiseToolVersionInRow(version string) bool {
	year, ok := miseCalverYear(version)
	return ok && year == miseToolSupportedCalverYear
}

func miseCalverYear(version string) (string, bool) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "", false
	}
	year, _, found := strings.Cut(version, ".")
	if !found || year == "" {
		return "", false
	}
	for _, r := range year {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return year, true
}
