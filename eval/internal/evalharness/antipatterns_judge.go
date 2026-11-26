package evalharness

import (
	"fmt"
	"sort"
	"strings"
)

// Results of one variant.
const (
	resultPass       = "pass"
	resultFail       = "fail"
	resultNotCovered = "not_covered"
	resultNowCovered = "now_covered"
)

// variantReport is one variant's row of the table: what was expected, what Coach reported, and the verdict.
type variantReport struct {
	Scan     string            `json:"scan"`
	Expected []expectedFinding `json:"expected"`
	Actual   []observedFinding `json:"actual"`
	Result   string            `json:"result"`
	Detail   string            `json:"detail,omitempty"`
}

func judge(status string, expected []expectedFinding, scan scanResult) variantReport {
	report := variantReport{Scan: scan.scan, Expected: nonNil(expected), Actual: nonNil(scan.findings)}
	missing, unexpected := difference(expected, scan.findings)
	switch {
	case status == statusNotCovered && len(scan.findings) == 0:
		report.Result = resultNotCovered
	case status == statusNotCovered:
		report.Result = resultNowCovered
	case len(missing) == 0 && len(unexpected) == 0:
		report.Result = resultPass
	default:
		report.Result = resultFail
		report.Detail = describe(missing, unexpected)
	}
	return report
}

func nonNil[T any](list []T) []T {
	if list == nil {
		return []T{}
	}
	return list
}

// difference is the expected findings Coach did not report, and the reported ones nobody expected.
func difference(expected []expectedFinding, actual []observedFinding) (missing, unexpected []expectedFinding) {
	want := map[expectedFinding]bool{}
	for _, finding := range expected {
		want[finding] = true
	}
	got := map[expectedFinding]bool{}
	for _, finding := range actual {
		got[finding] = true
		if !want[finding] {
			unexpected = append(unexpected, finding)
		}
	}
	for _, finding := range expected {
		if !got[finding] {
			missing = append(missing, finding)
		}
	}
	return missing, unexpected
}

func describe(missing, unexpected []expectedFinding) string {
	var parts []string
	for _, finding := range missing {
		parts = append(parts, fmt.Sprintf("missing %s at %s (%s)", finding.RuleID, finding.Path, finding.Subject))
	}
	for _, finding := range unexpected {
		parts = append(parts, fmt.Sprintf("unexpected %s at %s (%s)", finding.RuleID, finding.Path, finding.Subject))
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}
