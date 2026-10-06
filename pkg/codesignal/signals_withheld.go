package codesignal

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrInvalidSignalsWithheld reports a SignalsWithheld that Narrow cannot
// produce, which would encode as a view that drops or misstates what it
// withheld.
var ErrInvalidSignalsWithheld = errors.New("codesignal: invalid signals_withheld record")

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

func (w SignalsWithheld) validate() error {
	switch {
	case w.Top < 0 || w.BelowMinSeverity < 0 || w.BeyondTop < 0:
		return fmt.Errorf("%w: negative cap or count", ErrInvalidSignalsWithheld)
	case w.MinSeverity == "" && w.BelowMinSeverity != 0:
		return fmt.Errorf("%w: floor count without a floor", ErrInvalidSignalsWithheld)
	case w.Top == 0 && w.BeyondTop != 0:
		return fmt.Errorf("%w: cap count without a cap", ErrInvalidSignalsWithheld)
	case w.MinSeverity == "" && w.Top == 0:
		return fmt.Errorf("%w: no narrowing named", ErrInvalidSignalsWithheld)
	}
	return nil
}

// MarshalJSON emits each narrowing's pair of fields only while that narrowing
// is in effect, so a cap-only view carries no floor fields and vice versa. A
// record that names no narrowing, counts withheld signals for one it does not
// name, or holds a negative cap or count, is an error: Narrow cannot produce it,
// and encoded it would drop or misstate what the view withheld.
func (w SignalsWithheld) MarshalJSON() ([]byte, error) {
	if err := w.validate(); err != nil {
		return nil, err
	}
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
