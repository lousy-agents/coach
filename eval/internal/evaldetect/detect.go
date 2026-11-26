package evaldetect

import "errors"

var errNotImplemented = errors.New("evaldetect: not implemented")

// ErrGocognitUnavailable marks a pinned gocognit that cannot be built offline.
var ErrGocognitUnavailable = errors.New("evaldetect: pinned gocognit unavailable offline")

type Finding struct {
	RuleID  string
	Path    string
	Subject string
	Line    int
}

type Policy struct{}

func LoadPolicy(path string) (Policy, error) { return Policy{}, errNotImplemented }

func GoLayerViolations(root string, policy Policy) ([]Finding, error) {
	return nil, errNotImplemented
}

type Gocognit struct {
	Binary  string
	Version string
}

func (g Gocognit) CheckVersion() error { return errNotImplemented }

func (g Gocognit) Findings(root string) ([]Finding, error) { return nil, errNotImplemented }

func ResolveGocognit(evalModuleDir string, env []string) (Gocognit, error) {
	return Gocognit{}, errNotImplemented
}
