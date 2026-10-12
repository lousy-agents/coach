package codesignalcli

import "context"

// identityHandoffBackend answers every request with a fixed result, so a
// spec observes exactly what applyProjectBackend does with it.
type identityHandoffBackend struct {
	result *ProjectBackendResult
}

func (b identityHandoffBackend) Analyze(context.Context, ProjectBackendRequest) (*ProjectBackendResult, error) {
	return b.result, nil
}
