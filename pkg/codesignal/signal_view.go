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

func (r *Report) withheldSoFar() SignalsWithheld {
	if r.SignalsWithheld == nil {
		return SignalsWithheld{}
	}
	return *r.SignalsWithheld
}

// WithMinSeverity returns a copy of r whose Signals and ProjectChanges hold
// only entries at or above floor. Summary, Coverage, Diagnostics and every
// project coverage/summary block are shared with r untouched, so they keep
// describing the full analysis.
//
// A floor ParseSeverityFloor rejects returns r unchanged, so a bad value can
// never record a narrowing that does not match what was kept. Narrowings
// compose in either order: counts already withheld by r are kept, and the
// stricter of an existing and the new floor is reported.
//
// Project changes are mirrored one-to-one in Signals, so SignalsWithheld counts
// each withheld finding once and the project section narrows by the same rule.
func (r *Report) WithMinSeverity(floor Severity) *Report {
	if _, ok := ParseSeverityFloor(string(floor)); !ok {
		return r
	}
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

	record := r.withheldSoFar()
	if record.MinSeverity == "" || severityRank(floor) > severityRank(record.MinSeverity) {
		record.MinSeverity = floor
	}
	record.BelowMinSeverity += withheld
	narrowed.SignalsWithheld = &record
	return &narrowed
}

// WithTop returns a copy of r whose Signals hold only the first n entries, the
// highest-ranked because Build sorts them. Project changes follow their
// mirrored signal, so a capped-out project finding leaves ProjectChanges too
// and BeyondTop counts each finding once. Summary, Coverage and the other
// full-analysis blocks are shared with r untouched.
//
// An n below 1 is no cap and returns r unchanged, so a bad value can never
// read as an empty analysis. Narrowings compose in either order: counts
// already withheld by r are kept, BeyondTop accumulates, and the tighter of an
// existing and the new cap is reported.
func (r *Report) WithTop(n int) *Report {
	if n < 1 {
		return r
	}
	narrowed := *r

	kept := min(n, len(r.Signals))
	narrowed.Signals = append(make([]Signal, 0, kept), r.Signals[:kept]...)

	if r.ProjectChanges != nil {
		keptIDs := make(map[string]struct{}, kept)
		for _, signal := range narrowed.Signals {
			keptIDs[signal.ID] = struct{}{}
		}
		narrowed.ProjectChanges = make([]ProjectChange, 0, len(r.ProjectChanges))
		for _, change := range r.ProjectChanges {
			if _, ok := keptIDs[change.ID]; ok {
				narrowed.ProjectChanges = append(narrowed.ProjectChanges, change)
			}
		}
	}

	withheld := r.withheldSoFar()
	if withheld.Top == 0 || n < withheld.Top {
		withheld.Top = n
	}
	withheld.BeyondTop += len(r.Signals) - kept
	narrowed.SignalsWithheld = &withheld
	return &narrowed
}
