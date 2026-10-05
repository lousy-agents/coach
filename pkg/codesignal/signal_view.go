package codesignal

import (
	"encoding/json"
	"errors"
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

var (
	// ErrUnknownSeverityFloor reports a NarrowOptions.MinSeverity outside the
	// severities a Report can emit.
	ErrUnknownSeverityFloor = errors.New("codesignal: unknown severity floor")
	// ErrNegativeTop reports a NarrowOptions.Top below zero.
	ErrNegativeTop = errors.New("codesignal: negative top")
	// ErrAlreadyNarrowed reports a Narrow of a Report that already carries a
	// withheld record: a second narrowing could not say which view it produced.
	ErrAlreadyNarrowed = errors.New("codesignal: report is already narrowed")
)

// NarrowOptions selects a presentation view of a Report. The zero value
// narrows nothing: an empty MinSeverity is no floor and a zero Top is no cap.
type NarrowOptions struct {
	MinSeverity Severity
	Top         int
}

// Narrow returns a copy of r whose Signals and ProjectChanges hold only what
// opts keeps, and whose SignalsWithheld records what it left out. r must be
// non-nil and unnarrowed.
//
// The severity floor applies first and the cap takes the leading signals of
// what remains. Signals sort by lifecycle group before severity, so the two
// narrowings do not commute; Narrow is the only entry point and fixes the
// order, so the record always describes the view that was produced.
//
// Summary, Coverage, Diagnostics and every project summary and coverage block
// are shared with r untouched. ProjectChanges follow their mirrored signal by
// ID, so each finding is counted once.
//
// Zero opts return r itself with no record. An unrecognised floor, a negative
// Top, or an already narrowed r returns an error and no view, never a
// different view than the one asked for.
func (r *Report) Narrow(opts NarrowOptions) (*Report, error) {
	if opts == (NarrowOptions{}) {
		return r, nil
	}
	if err := r.validateNarrowing(opts); err != nil {
		return nil, err
	}

	var withheld SignalsWithheld
	kept := r.Signals
	if opts.MinSeverity != "" {
		withheld.MinSeverity = opts.MinSeverity
		kept, withheld.BelowMinSeverity = atOrAboveFloor(kept, opts.MinSeverity)
	}
	if opts.Top > 0 {
		withheld.Top = opts.Top
		kept, withheld.BeyondTop = leading(kept, opts.Top)
	}

	narrowed := *r
	narrowed.Signals = kept
	narrowed.ProjectChanges = followSignals(r.ProjectChanges, kept)
	narrowed.SignalsWithheld = &withheld
	return &narrowed, nil
}

func (r *Report) validateNarrowing(opts NarrowOptions) error {
	if r.SignalsWithheld != nil {
		return ErrAlreadyNarrowed
	}
	if opts.Top < 0 {
		return ErrNegativeTop
	}
	if opts.MinSeverity != "" && !slices.Contains(severityOrder, opts.MinSeverity) {
		return ErrUnknownSeverityFloor
	}
	return nil
}

func atOrAboveFloor(signals []Signal, floor Severity) ([]Signal, int) {
	kept := make([]Signal, 0, len(signals))
	floorRank := severityRank(floor)
	for _, signal := range signals {
		if severityRank(signal.Severity) >= floorRank {
			kept = append(kept, signal)
		}
	}
	return kept, len(signals) - len(kept)
}

func leading(signals []Signal, n int) ([]Signal, int) {
	kept := min(n, len(signals))
	return append(make([]Signal, 0, kept), signals[:kept]...), len(signals) - kept
}

func followSignals(changes []ProjectChange, kept []Signal) []ProjectChange {
	if changes == nil {
		return nil
	}
	keptIDs := make(map[string]struct{}, len(kept))
	for _, signal := range kept {
		keptIDs[signal.ID] = struct{}{}
	}
	followed := make([]ProjectChange, 0, len(changes))
	for _, change := range changes {
		if _, ok := keptIDs[change.ID]; ok {
			followed = append(followed, change)
		}
	}
	return followed
}
