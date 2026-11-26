package evaldetect

import "errors"

var errNotImplemented = errors.New("evaldetect: not implemented")

var ErrRefused = errors.New("evaldetect: refused")

func TSLayerViolations(root string, policy Policy) ([]Finding, error) {
	return nil, errNotImplemented
}
