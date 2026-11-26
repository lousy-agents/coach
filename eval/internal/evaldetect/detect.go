package evaldetect

import "errors"

// ErrRefused marks input a detector does not support and will not guess at.
var ErrRefused = errors.New("evaldetect: refused")

// ErrGocognitUnavailable marks a pinned gocognit that cannot be built offline.
var ErrGocognitUnavailable = errors.New("evaldetect: pinned gocognit unavailable offline")

const layerViolation = "architecture.layer_violation"

// Finding is one graded issue: the rule, the file and line it is in, and a
// subject that keys it for comparison with Coach's own report.
type Finding struct {
	RuleID  string
	Path    string
	Subject string
	Line    int
}
