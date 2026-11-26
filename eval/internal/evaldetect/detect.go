package evaldetect

import "errors"

var errNotImplemented = errors.New("evaldetect: not implemented")

// Finding is one graded issue: the rule, the file and line it is in, and a
// subject that keys it for comparison with Coach's own report.
type Finding struct {
	RuleID  string
	Path    string
	Subject string
	Line    int
}

// Policy is the layer model a detector reads: the same .lousy-agents/coach.json
// the self-review hook reads.
type Policy struct{}

func LoadPolicy(path string) (Policy, error) { return Policy{}, errNotImplemented }

func GoLayerViolations(root string, policy Policy) ([]Finding, error) {
	return nil, errNotImplemented
}

func TSLayerViolations(root string, policy Policy, typescriptDir string) ([]Finding, error) {
	return nil, errNotImplemented
}

// Gocognit runs a pinned gocognit binary.
type Gocognit struct {
	Binary  string
	Version string
}

func (g Gocognit) CheckVersion() error { return errNotImplemented }

func (g Gocognit) Findings(root string) ([]Finding, error) { return nil, errNotImplemented }
