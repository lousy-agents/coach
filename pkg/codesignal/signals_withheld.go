package codesignal

import (
	"encoding/json"
	"slices"
)

// SignalsWithheld records what a narrowed view of a Report left out, so a
// shortened signals list is never mistaken for a smaller analysis. It is nil on
// an unnarrowed Report. A narrowing is in effect when its identifying field is
// set: MinSeverity for the severity floor, Top for the cap; a zero count next to
// a set identifier is still reported.
//
// It describes presentation narrowing only: Summary and Coverage keep
// describing the full analysis. An analysis-level filter (suppression, vendor
// exclusion) changes what was analyzed, so it must change Summary and Coverage
// and shall not reuse this record.
type SignalsWithheld struct {
	MinSeverity      Severity `json:"min_severity,omitempty"`
	BelowMinSeverity int      `json:"below_min_severity"`
	Top              int      `json:"top,omitempty"`
	BeyondTop        int      `json:"beyond_top"`
}

// MarshalJSON emits each narrowing's pair of fields only while that narrowing
// is in effect, so a cap-only view carries no floor fields and vice versa.
func (w SignalsWithheld) MarshalJSON() ([]byte, error) {
	var wire struct {
		MinSeverity      Severity `json:"min_severity,omitempty"`
		BelowMinSeverity *int     `json:"below_min_severity,omitempty"`
		Top              int      `json:"top,omitempty"`
		BeyondTop        *int     `json:"beyond_top,omitempty"`
	}
	if w.MinSeverity != "" {
		wire.MinSeverity = w.MinSeverity
		wire.BelowMinSeverity = &w.BelowMinSeverity
	}
	if w.Top > 0 {
		wire.Top = w.Top
		wire.BeyondTop = &w.BeyondTop
	}
	return json.Marshal(wire)
}

// ParseSeverityFloor accepts exactly the severities the report can emit.
func ParseSeverityFloor(value string) (Severity, bool) {
	if floor := Severity(value); slices.Contains(severityOrder, floor) {
		return floor, true
	}
	return "", false
}
