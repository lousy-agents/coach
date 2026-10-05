package codesignalcli

import (
	"strings"
)

func formatOriginFindings(findings []ReadinessOriginFinding) string {
	if len(findings) == 0 {
		return ""
	}
	parts := make([]string, 0, len(findings))
	for _, finding := range findings {
		parts = append(parts, finding.Origin+":"+finding.Class)
	}
	return strings.Join(parts, ",")
}
