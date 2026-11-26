package evalhistory

import (
	"fmt"
	"math"
)

// z95 is the two-sided 95% normal quantile.
const z95 = 1.959963985

// Association is a flagged-vs-unflagged risk ratio with its Katz log 95%
// interval. It is an association, never a causal effect, precision or
// recall. Katz's interval needs all four cells non-zero, so when any is
// zero the ratio is null and Reason says why.
type Association struct {
	RiskRatio *float64
	Low       *float64
	High      *float64
	Reason    string
}

// RiskRatio compares the share of flagged units a later fix involved with
// the share of controls it involved.
func RiskRatio(flaggedInvolved, flagged, controlInvolved, controls int) Association {
	switch {
	case flagged == 0:
		return Association{Reason: "no flagged unit"}
	case controls == 0:
		return Association{Reason: "no control"}
	case flaggedInvolved == 0 || controlInvolved == 0:
		return Association{Reason: fmt.Sprintf("zero cell: %d of %d flagged and %d of %d controls involved", flaggedInvolved, flagged, controlInvolved, controls)}
	case flaggedInvolved == flagged && controlInvolved == controls:
		return Association{Reason: "no variance: every unit involved"}
	case flaggedInvolved == flagged:
		return Association{Reason: "no variance: every flagged unit involved"}
	case controlInvolved == controls:
		return Association{Reason: "no variance: every control involved"}
	}
	ratio := (float64(flaggedInvolved) / float64(flagged)) / (float64(controlInvolved) / float64(controls))
	se := math.Sqrt(1/float64(flaggedInvolved) - 1/float64(flagged) + 1/float64(controlInvolved) - 1/float64(controls))
	low, high := ratio*math.Exp(-z95*se), ratio*math.Exp(z95*se)
	return Association{RiskRatio: &ratio, Low: &low, High: &high}
}
