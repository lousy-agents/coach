package codesignalcli

import (
	"fmt"
)

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
