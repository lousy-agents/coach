package codesignal

// SignalsWithheld records what a narrowed view of a Report left out, so a
// shortened signals list is never mistaken for a smaller analysis. It is nil on
// an unnarrowed Report.
type SignalsWithheld struct {
	MinSeverity      Severity `json:"min_severity,omitempty"`
	BelowMinSeverity int      `json:"below_min_severity"`
}

// ParseSeverityFloor accepts exactly the severities the report can emit.
func ParseSeverityFloor(value string) (Severity, bool) {
	switch floor := Severity(value); floor {
	case "high", "medium", "advisory", "low":
		return floor, true
	}
	return "", false
}

// WithMinSeverity returns a copy of r whose Signals and ProjectChanges hold
// only entries at or above floor. Summary, Coverage, Diagnostics and every
// project coverage/summary block are shared with r untouched, so they keep
// describing the full analysis.
//
// Project changes are mirrored one-to-one in Signals, so SignalsWithheld counts
// each withheld finding once and the project section narrows by the same rule.
func (r *Report) WithMinSeverity(floor Severity) *Report {
	narrowed := *r
	floorRank := severityRank(floor)

	narrowed.Signals = make([]Signal, 0, len(r.Signals))
	withheld := 0
	for _, signal := range r.Signals {
		if severityRank(signal.Severity) < floorRank {
			withheld++
			continue
		}
		narrowed.Signals = append(narrowed.Signals, signal)
	}

	if r.ProjectChanges != nil {
		narrowed.ProjectChanges = make([]ProjectChange, 0, len(r.ProjectChanges))
		for _, change := range r.ProjectChanges {
			if severityRank(change.Severity) >= floorRank {
				narrowed.ProjectChanges = append(narrowed.ProjectChanges, change)
			}
		}
	}

	narrowed.SignalsWithheld = &SignalsWithheld{MinSeverity: floor, BelowMinSeverity: withheld}
	return &narrowed
}
