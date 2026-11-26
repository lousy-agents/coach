package evaldetect

import "errors"

var errNotImplemented = errors.New("evaldetect: not implemented")

const layerViolation = "architecture.layer_violation"

// Finding is one graded issue: the rule, the file and line it is in, and a
// subject that keys it for comparison with Coach's own report.
type Finding struct {
	RuleID  string
	Path    string
	Subject string
	Line    int
}

func TSLayerViolations(root string, policy Policy, typescriptDir string) ([]Finding, error) {
	return nil, errNotImplemented
}
