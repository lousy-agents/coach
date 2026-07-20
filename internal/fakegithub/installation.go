package fakegithub

import (
	"net/http"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// installationTokenHandler answers
// POST /api/v3/app/installations/{id}/access_tokens. Stub: full behavior
// lands with a later Task 0.2 slice.
func installationTokenHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fakegithub: installationTokenHandler not yet implemented (Task 0.2 in progress)", http.StatusNotImplemented)
	}
}

// installationResolutionHandler answers
// GET /api/v3/repos/{owner}/{repo}/installation. Stub: full behavior lands
// with a later Task 0.2 slice.
func installationResolutionHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fakegithub: installationResolutionHandler not yet implemented (Task 0.2 in progress)", http.StatusNotImplemented)
	}
}

// permissionHandler answers
// GET /api/v3/repos/{owner}/{repo}/collaborators/{username}/permission.
// Stub: full behavior lands with a later Task 0.2 slice.
func permissionHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fakegithub: permissionHandler not yet implemented (Task 0.2 in progress)", http.StatusNotImplemented)
	}
}
