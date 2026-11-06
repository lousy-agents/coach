package render

import (
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
	line := terminalText(gap.Code)
	if gap.PackageManagerKind != "" {
		line += " package_manager_kind=" + terminalText(gap.PackageManagerKind)
	}
	if gap.Cause != "" {
		line += " cause=" + terminalText(string(gap.Cause))
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

// readinessCheckField is one "label=value" of a check line. Its value is already safe to print: every
// value comes from a repository or from what is installed in it, so readinessCheckFields passes each one
// through terminalText.
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
		{label: "kind", value: terminalText(check.Kind)},
		{label: "version", value: terminalText(check.Version)},
		{label: "origin", value: terminalText(check.Origin)},
		{label: "expected_version", value: terminalText(check.ExpectedVersion)},
		{label: "found_version", value: terminalText(check.FoundVersion)},
		{label: "pinned_version", value: terminalText(check.PinnedVersion)},
		{label: "declared_version", value: terminalText(declaredVersion)},
		{label: "supported_versions", value: terminalList(check.SupportedVersions, ",")},
		{label: "root_findings", value: terminalText(projectreadiness.FormatRootFindings(check.RootFindings))},
		{label: "detail", value: terminalText(check.Detail)},
		{label: "origins", value: terminalText(projectreadiness.FormatOriginFindings(check.OriginFindings))},
	}
}
