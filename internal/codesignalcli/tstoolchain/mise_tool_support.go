package tstoolchain

import (
	"context"
	"fmt"
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// miseToolSupportedCalverYear is the calver year of the mise release this
// codebase is tested against, per the frozen mise-origin row: "supported
// mise 2026.x (calver year of the tested release)". mise releases use
// calver (YYYY.M.P); the tested release is whatever mise.toml already pins
// via min_version, not a value chosen during coding. This constant must be
// updated together with mise.toml's min_version year -- see
// TestMiseToolSupportedCalverYearTracksMiseToml.
const miseToolSupportedCalverYear = "2026"

const MiseNpmTypescriptTool = "npm:typescript"

// miseInstallCommandTemplate and miseWhereCommand are the frozen mise-origin
// row's exact command sequence: install the exact resolved version, then
// locate it with `mise where` (miseInstallToolSpec and
// LocateMiseTypescriptInstall use this same sequence).
const miseInstallCommandTemplate = "mise install " + MiseNpmTypescriptTool + "@%s"

const miseWhereCommand = "mise where"

// MiseBackingNpmSuppressionFlag is the same suppression flag the frozen npm
// adapter row uses: the mise-origin row installs npm:typescript via npm's
// own resolution under the hood, so its backing npm invocation must satisfy
// this same suppression, proven with the same sentinel fixture, rather than
// a mise-specific mechanism invented separately.
const MiseBackingNpmSuppressionFlag = "--ignore-scripts"

// MiseInstallCommand renders the frozen row's install command for an exact
// resolved version. The frozen row names exactly one version per
// installation -- never a secondary or fallback version -- so this takes a
// single version string, not a collection.
func MiseInstallCommand(version string) string {
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

// miseToolReadiness classifies whether the `mise` tool binary itself is a
// supported version -- a prerequisite gate answered before mise's own
// TypeScript-version detection is trusted, and before mise is offered as a
// prepare_compiler installation choice. This is deliberately distinct from
// ClassEligible/Unsupported in project_ts_compiler_aggregate.go,
// which classify the *TypeScript* version mise installed -- a different
// question about a different piece of software. Ready is true only when the
// detected version falls within the frozen row (isMiseToolVersionInRow);
// otherwise Code names which of the two split fail-closed gaps applies:
// projectreadiness.GapPackageManagerVersionUnverifiable when the version could not be
// determined at all, projectreadiness.GapPackageManagerVersionUnsupported when it was
// determined but falls outside the row.
type miseToolReadiness struct {
	ready bool
	code  string
}

// evaluateMiseToolVersionReadiness probes and classifies the mise tool's own
// version. It never distinguishes "mise absent from PATH" from "mise present
// but --version failed or produced nothing parseable" -- both fail closed to
// projectreadiness.GapPackageManagerVersionUnverifiable.
func evaluateMiseToolVersionReadiness(ctx context.Context) miseToolReadiness {
	version, ok := ProbeMiseToolVersion(ctx)
	if !ok {
		return miseToolReadiness{code: projectreadiness.GapPackageManagerVersionUnverifiable}
	}
	token, ok := miseToolVersionToken(version)
	if !ok {
		return miseToolReadiness{code: projectreadiness.GapPackageManagerVersionUnverifiable}
	}
	if !isMiseToolVersionInRow(token) {
		return miseToolReadiness{code: projectreadiness.GapPackageManagerVersionUnsupported}
	}
	return miseToolReadiness{ready: true}
}

// miseToolVersionToken extracts the leading calver token from `mise
// --version`'s output, e.g. "2026.9.5" from "2026.9.5 linux-x64
// (2026-09-10)". ok is false for empty or all-whitespace output.
func miseToolVersionToken(versionOutput string) (string, bool) {
	trimmed := strings.TrimSpace(versionOutput)
	if trimmed == "" {
		return "", false
	}
	token, _, _ := strings.Cut(trimmed, " ")
	if token == "" {
		return "", false
	}
	return token, true
}
