package projectreadiness

import (
	"strings"
)

func FormatRootFindings(findings []RootFinding) string {
	if len(findings) == 0 {
		return ""
	}
	parts := make([]string, 0, len(findings))
	for _, finding := range findings {
		parts = append(parts, formatRootFinding(finding))
	}
	return strings.Join(parts, ",")
}

func formatRootFinding(finding RootFinding) string {
	if finding.Version == "" {
		return finding.Root
	}
	return finding.Root + "@" + finding.Version
}

func FormatOriginFindings(findings []OriginFinding) string {
	if len(findings) == 0 {
		return ""
	}
	parts := make([]string, 0, len(findings))
	for _, finding := range findings {
		parts = append(parts, finding.Origin+":"+finding.Class)
	}
	return strings.Join(parts, ",")
}
