package codesignalcli

import (
	"context"
	"strings"
)

// miseToolReadiness classifies whether the `mise` tool binary itself is a
// supported version -- a prerequisite gate answered before mise's own
// TypeScript-version detection (project_ts_compiler_mise.go) is trusted, and
// before mise is offered as a prepare_compiler installation choice
// (SA-280-045). This is deliberately distinct from compilerClassEligible/
// Unsupported in project_ts_compiler_aggregate.go, which classify the
// *TypeScript* version mise installed -- a different question about a
// different piece of software. Ready is true only when the detected version
// falls within the frozen row (isMiseToolVersionInRow); otherwise Code names
// which of the two split fail-closed gaps applies (SA-280-015 AC-11):
// GapPackageManagerVersionUnverifiable when the version could not be
// determined at all, GapPackageManagerVersionUnsupported when it was
// determined but falls outside the row.
type miseToolReadiness struct {
	ready bool
	code  string
}

// evaluateMiseToolVersionReadiness probes and classifies the mise tool's own
// version. It never distinguishes "mise absent from PATH" from "mise present
// but --version failed or produced nothing parseable" -- both fail closed to
// GapPackageManagerVersionUnverifiable, per AC-11.
func evaluateMiseToolVersionReadiness(ctx context.Context) miseToolReadiness {
	version, ok := probeMiseToolVersion(ctx)
	if !ok {
		return miseToolReadiness{code: GapPackageManagerVersionUnverifiable}
	}
	token, ok := miseToolVersionToken(version)
	if !ok {
		return miseToolReadiness{code: GapPackageManagerVersionUnverifiable}
	}
	if !isMiseToolVersionInRow(token) {
		return miseToolReadiness{code: GapPackageManagerVersionUnsupported}
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
