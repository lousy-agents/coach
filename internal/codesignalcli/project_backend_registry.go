package codesignalcli

import (
	"fmt"
)

// ProjectBackendUnavailableError signals a valid project configuration whose
// requested language has no registered project-analysis backend. It maps to
// exit code 3 and is reported in the local CodeSignal document.
type ProjectBackendUnavailableError struct {
	Message string
}

func (e *ProjectBackendUnavailableError) Error() string { return e.Message }

// ResolveProjectBackend reports whether a project-analysis backend is
// registered for language. "go" and "typescript" both have registered
// backends today; every other language, including the empty string, remains
// unavailable until its own backend lands.
func ResolveProjectBackend(language string) error {
	if language == "go" || language == "typescript" {
		return nil
	}
	return &ProjectBackendUnavailableError{Message: fmt.Sprintf("coach codesignal: no project-analysis backend is available for language %q yet (project_backend_unavailable)", language)}
}
