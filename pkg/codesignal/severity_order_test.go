package codesignal

import "testing"

func TestSeverityOrderBacksRankAndFloorParsing(t *testing.T) {
	ranks := map[int]Severity{}
	for _, severity := range severityOrder {
		if other, dup := ranks[severityRank(severity)]; dup {
			t.Fatalf("severities %q and %q share rank %d", other, severity, severityRank(severity))
		}
		ranks[severityRank(severity)] = severity

		if got, ok := ParseSeverityFloor(string(severity)); !ok || got != severity {
			t.Fatalf("ParseSeverityFloor(%q) = %q, %t; every ranked severity must be an accepted floor", severity, got, ok)
		}
	}
	for _, notSeverity := range []string{"", "bogus", "HIGH", " high"} {
		if _, ok := ParseSeverityFloor(notSeverity); ok {
			t.Fatalf("ParseSeverityFloor(%q) accepted a value outside severityOrder", notSeverity)
		}
	}
	for i := 1; i < len(severityOrder); i++ {
		if severityRank(severityOrder[i]) <= severityRank(severityOrder[i-1]) {
			t.Fatalf("severityOrder must be lowest priority first: %q does not outrank %q", severityOrder[i], severityOrder[i-1])
		}
	}
}
