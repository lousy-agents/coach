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
	resultMiss       = "miss"
	resultNowCovered = "now_covered"
	resultIncomplete = "incomplete"
)

// variantReport is one variant's row of the table: what was expected, what Coach reported, and the verdict.
type variantReport struct {
	Scan             string            `json:"scan"`
	Expected         []expectedFinding `json:"expected"`
	Actual           []observedFinding `json:"actual"`
	CoverageComplete bool              `json:"coverage_complete"`
	Diagnostics      []string          `json:"diagnostics"`
	Result           string            `json:"result"`
	Detail           string            `json:"detail,omitempty"`
	// ok is false when the variant makes the run exit 1.
	ok bool
}

// judge compares what coach reported on a variant with what the fixture expects of it. incompleteExpected says that the
// fixture declared this scan incomplete.
func judge(status string, expected []expectedFinding, scan scanResult, incompleteExpected bool) variantReport {
	report := variantReport{
		Scan: scan.scan, Expected: nonNil(expected), Actual: nonNil(scan.findings),
		CoverageComplete: scan.complete, Diagnostics: nonNil(scan.diagnostics), ok: true,
	}
	switch {
	case !scan.complete && !incompleteExpected:
		return report.failed(resultIncomplete, "the scan was incomplete ("+strings.Join(scan.diagnostics, ", ")+"), so a silent result proves nothing")
	case scan.complete && incompleteExpected:
		return report.failed(resultFail, "the fixture expects an incomplete scan, but coverage was complete")
	case status == statusNotCovered:
		return report.knownMiss(scan, incompleteExpected)
	}
	missing, unexpected := difference(expected, scan.findings)
	if len(missing) == 0 && len(unexpected) == 0 {
		report.Result = resultPass
		return report
	}
	return report.failed(resultFail, describe(missing, unexpected))
}

func (r variantReport) failed(result, detail string) variantReport {
	r.Result, r.Detail, r.ok = result, detail, false
	return r
}

// knownMiss is the result of a variant of a fixture Coach has no rule for: a miss when it is silent on a complete scan,
// incomplete when the fixture says the scan is, and now_covered when Coach starts to report on it.
func (r variantReport) knownMiss(scan scanResult, incompleteExpected bool) variantReport {
	switch {
	case len(scan.findings) > 0:
		r.Result = resultNowCovered
	case incompleteExpected:
		r.Result = resultIncomplete
	default:
		r.Result = resultMiss
	}
	return r
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
