package fakegithub

import (
	"net/http"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// oauthAuthorizeHandler answers GET /login/oauth/authorize. Stub: full
// behavior lands with a later Task 0.2 slice.
func oauthAuthorizeHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fakegithub: oauthAuthorizeHandler not yet implemented (Task 0.2 in progress)", http.StatusNotImplemented)
	}
}

// oauthTokenHandler answers POST /login/oauth/access_token. Stub: full
// behavior lands with a later Task 0.2 slice.
func oauthTokenHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fakegithub: oauthTokenHandler not yet implemented (Task 0.2 in progress)", http.StatusNotImplemented)
	}
}

// oauthUserHandler answers GET /user. Stub: full behavior lands with a
// later Task 0.2 slice.
func oauthUserHandler(fx *Fixture, rec *acceptanceharness.Recorder) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "fakegithub: oauthUserHandler not yet implemented (Task 0.2 in progress)", http.StatusNotImplemented)
	}
}
