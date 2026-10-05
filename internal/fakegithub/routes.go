package fakegithub

import (
	"net/http"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// Handler builds the http.Handler and Recorder that answer fixture's OAuth,
// installation, and contents routes -- the same route table NewServer wraps
// in an httptest.Server. Callers that need the fake embedded in their own
// server topology (e.g. a standalone process serving real HTTP, rather than
// an in-process httptest.Server) can use Handler directly. fixture must
// outlive the returned handler (it is not copied). Panics if fixture is nil.
func Handler(fixture *Fixture) (http.Handler, *acceptanceharness.Recorder) {
	requireFixture("Handler", fixture)

	rec := &acceptanceharness.Recorder{}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /login/oauth/authorize", oauthAuthorizeHandler(fixture, rec))
	mux.HandleFunc("POST /login/oauth/access_token", oauthTokenHandler(fixture, rec))
	// /user is github.com's public path (raw net/http tests).
	// /api/v3/user is what go-github emits with WithEnterpriseURLs (api/v3 prefix).
	// OAuth authorize/token stay on bare paths; App/repos APIs use api/v3.
	mux.HandleFunc("GET /user", oauthUserHandler(fixture, rec))
	mux.HandleFunc("GET /api/v3/user", oauthUserHandler(fixture, rec))

	mux.HandleFunc("POST /api/v3/app/installations/{id}/access_tokens", installationTokenHandler(fixture, rec))
	mux.HandleFunc("GET /api/v3/repos/{owner}/{repo}/installation", installationResolutionHandler(fixture, rec))
	mux.HandleFunc("GET /api/v3/repos/{owner}/{repo}/collaborators/{username}/permission", permissionHandler(fixture, rec))
	mux.HandleFunc("GET /api/v3/repos/{owner}/{repo}/contents/{path...}", contentsHandler(fixture, rec))
	// Register commits before bare repo so the more-specific path wins.
	mux.HandleFunc("GET /api/v3/repos/{owner}/{repo}/commits/{ref}", commitHandler(fixture, rec))
	mux.HandleFunc("GET /api/v3/repos/{owner}/{repo}", repoMetaHandler(fixture, rec))

	return mux, rec
}
