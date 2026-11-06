package render

import (
	"strings"

	"github.com/lousy-agents/coach/internal/codesignalcli/projectreadiness"
)

// gapCodes renders each gap for ReadinessText's Gaps list. A gap
// carrying PackageManagerKind (an independent mise_project/mise_global
// choice sharing a code with the project adapter, or with each other)
// renders it alongside the code, so the text output does not collapse two
// distinct package-manager gaps into indistinguishable lines the way a
// bare code list would.
func gapCodes(gaps []projectreadiness.Gap) []string {
	codes := make([]string, len(gaps))
	for i, gap := range gaps {
		codes[i] = gapLine(gap)
	}
	return codes
}

// gapLine is a gap's code followed by the fields it carries, in the order the JSON
// declares them.
func gapLine(gap projectreadiness.Gap) string {
	line := gap.Code
	if gap.PackageManagerKind != "" {
		line += " package_manager_kind=" + gap.PackageManagerKind
	}
	if gap.Cause != "" {
		line += " cause=" + string(gap.Cause)
	}
	return line
}

func nextActionKinds(actions []projectreadiness.NextAction) []string {
	kinds := make([]string, len(actions))
	for i, action := range actions {
		kinds[i] = action.Kind
	}
	return kinds
}

type readinessCheckField struct {
	label string
	value string
}

func readinessCheckFields(check projectreadiness.Check) []readinessCheckField {
	declaredVersion := check.DeclaredVersion
	if check.State != projectreadiness.Fail {
		declaredVersion = ""
	}
	return []readinessCheckField{
		{label: "kind", value: check.Kind},
		{label: "version", value: check.Version},
		{label: "origin", value: check.Origin},
		{label: "expected_version", value: check.ExpectedVersion},
		{label: "found_version", value: check.FoundVersion},
		{label: "pinned_version", value: check.PinnedVersion},
		{label: "declared_version", value: declaredVersion},
		{label: "supported_versions", value: strings.Join(check.SupportedVersions, ",")},
		{label: "root_findings", value: projectreadiness.FormatRootFindings(check.RootFindings)},
		{label: "detail", value: check.Detail},
		{label: "origins", value: projectreadiness.FormatOriginFindings(check.OriginFindings)},
	}
}
