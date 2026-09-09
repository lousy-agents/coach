package codesignalcli

import "strings"

func gapCodes(gaps []ReadinessGap) []string {
	codes := make([]string, len(gaps))
	for i, gap := range gaps {
		codes[i] = gap.Code
	}
	return codes
}

func nextActionKinds(actions []ReadinessNextAction) []string {
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

func readinessCheckFields(check ReadinessCheck) []readinessCheckField {
	declaredVersion := check.DeclaredVersion
	if check.State != ReadinessFail {
		declaredVersion = ""
	}
	return []readinessCheckField{
		{label: "kind", value: check.Kind},
		{label: "version", value: check.Version},
		{label: "origin", value: check.Origin},
		{label: "expected_version", value: check.ExpectedVersion},
		{label: "found_version", value: check.FoundVersion},
		{label: "declared_version", value: declaredVersion},
		{label: "supported_versions", value: strings.Join(check.SupportedVersions, ",")},
		{label: "root_findings", value: formatRootFindings(check.RootFindings)},
		{label: "detail", value: check.Detail},
		{label: "origins", value: formatOriginFindings(check.OriginFindings)},
	}
}

func formatRootFindings(findings []ReadinessRootFinding) string {
	if len(findings) == 0 {
		return ""
	}
	parts := make([]string, 0, len(findings))
	for _, finding := range findings {
		parts = append(parts, formatRootFinding(finding))
	}
	return strings.Join(parts, ",")
}

func formatRootFinding(finding ReadinessRootFinding) string {
	if finding.Version == "" {
		return finding.Root
	}
	return finding.Root + "@" + finding.Version
}
