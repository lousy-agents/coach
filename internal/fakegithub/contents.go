package fakegithub

import (
	"net/http"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// contentsHandler answers
// GET /api/v3/repos/{owner}/{repo}/contents/{path...}. Stub: full behavior
// lands with a later Task 0.2 slice.
func contentsHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fakegithub: contentsHandler not yet implemented (Task 0.2 in progress)", http.StatusNotImplemented)
	}
}
