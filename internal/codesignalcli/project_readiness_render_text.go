package codesignalcli

import (
	"fmt"
	"strings"
)

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

func renderReadinessCodeList(b *strings.Builder, heading string, values []string) {
	if len(values) == 0 {
		return
	}
	fmt.Fprintf(b, "\n%s:\n", heading)
	for _, value := range values {
		fmt.Fprintf(b, "  %s\n", value)
	}
}

func renderReadinessWarnings(b *strings.Builder, warnings []ReadinessWarning) {
	if len(warnings) == 0 {
		return
	}
	b.WriteString("\nWarnings:\n")
	for _, warning := range warnings {
		renderReadinessWarningLine(b, warning)
	}
}

func renderReadinessWarningLine(b *strings.Builder, warning ReadinessWarning) {
	if warning.Code == WarnCompilerDeclarationMismatch {
		fmt.Fprintf(b, "  %s (declared_version=%s found_version=%s declaration_origin=%s)\n", warning.Code, warning.DeclaredVersion, warning.FoundVersion, warning.DeclarationOrigin)
		return
	}
	fmt.Fprintf(b, "  %s (found_major=%d tested_major=%d floor_major=%d)\n", warning.Code, warning.FoundMajor, warning.TestedMajor, warning.FloorMajor)
}

func renderReadinessDirtyWorktree(b *strings.Builder, dirty ReadinessDirtyWorktree) {
	if !dirty.RelevantChanges {
		return
	}
	b.WriteString("\nWarning: uncommitted or untracked changes exist under paths relevant to this result. ")
	b.WriteString("These paths are not part of the analyzed revision and had no effect on the checks above:\n")
	for _, changedPath := range dirty.Paths {
		fmt.Fprintf(b, "  %s\n", changedPath)
	}
}

func renderReadinessCheckLine(b *strings.Builder, name string, check ReadinessCheck) {
	fmt.Fprintf(b, "  %s: %s", name, check.State)
	if check.Code != "" {
		fmt.Fprintf(b, " (%s)", check.Code)
	}
	if check.Version != "" {
		fmt.Fprintf(b, " version=%s", check.Version)
	}
	if check.ExpectedVersion != "" {
		fmt.Fprintf(b, " expected_version=%s", check.ExpectedVersion)
	}
	if check.FoundVersion != "" {
		fmt.Fprintf(b, " found_version=%s", check.FoundVersion)
	}
	if check.State == ReadinessFail && check.DeclaredVersion != "" {
		fmt.Fprintf(b, " declared_version=%s", check.DeclaredVersion)
	}
	if len(check.SupportedVersions) > 0 {
		fmt.Fprintf(b, " supported_versions=%s", strings.Join(check.SupportedVersions, ","))
	}
	if formatted := formatRootFindings(check.RootFindings); formatted != "" {
		fmt.Fprintf(b, " root_findings=%s", formatted)
	}
	b.WriteString("\n")
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
