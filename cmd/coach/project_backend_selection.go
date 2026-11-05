package main

import (
	"errors"
	"fmt"

	"github.com/lousy-agents/coach/internal/codesignalcli"
)

// selectProjectBackend returns the backend that analyzes language. When none
// can run, backend is nil and unavailable says why; err is set only for a
// failure that is not "no backend is available".
func selectProjectBackend(ports scanPorts, language string) (backend codesignalcli.ProjectBackend, unavailable string, err error) {
	if err := ports.resolveBackend(language); err != nil {
		var backendErr *codesignalcli.ProjectBackendUnavailableError
		if !errors.As(err, &backendErr) {
			return nil, "", err
		}
		return nil, backendErr.Message, nil
	}
	backend = ports.lookupBackend(language)
	if backend == nil {
		return nil, fmt.Sprintf("coach codesignal: no project-analysis backend is available for language %q yet (project_backend_unavailable)", language), nil
	}
	return backend, "", nil
}
