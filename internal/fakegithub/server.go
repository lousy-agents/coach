package fakegithub

import (
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// Server is an in-process httptest.Server for the OAuth, installation, and
// contents routes this package implements. It is driven by a caller-supplied
// [Fixture] and records every handled request via acceptanceharness.Recorder.
//
// Route paths stay here; handler bodies live in oauth.go, installation.go,
// and contents.go.
type Server struct {
	http     *httptest.Server
	fixture  *Fixture
	recorder *acceptanceharness.Recorder
}

// NewServer starts a Server backed by fixture. fixture must outlive the
// Server (it is not copied). Callers must Close when done. Panics if fixture
// is nil.
func NewServer(fixture *Fixture) *Server {
	if fixture == nil {
		panic("fakegithub: NewServer called with a nil Fixture")
	}

	s := &Server{fixture: fixture, recorder: &acceptanceharness.Recorder{}}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /login/oauth/authorize", oauthAuthorizeHandler(s.fixture, s.recorder))
	mux.HandleFunc("POST /login/oauth/access_token", oauthTokenHandler(s.fixture, s.recorder))
	// /user is github.com's public path (raw net/http tests).
	// /api/v3/user is what go-github emits with WithEnterpriseURLs (api/v3 prefix).
	// OAuth authorize/token stay on bare paths; App/repos APIs use api/v3.
	mux.HandleFunc("GET /user", oauthUserHandler(s.fixture, s.recorder))
	mux.HandleFunc("GET /api/v3/user", oauthUserHandler(s.fixture, s.recorder))

	mux.HandleFunc("POST /api/v3/app/installations/{id}/access_tokens", installationTokenHandler(s.fixture, s.recorder))
	mux.HandleFunc("GET /api/v3/repos/{owner}/{repo}/installation", installationResolutionHandler(s.fixture, s.recorder))
	mux.HandleFunc("GET /api/v3/repos/{owner}/{repo}/collaborators/{username}/permission", permissionHandler(s.fixture, s.recorder))
	mux.HandleFunc("GET /api/v3/repos/{owner}/{repo}/contents/{path...}", contentsHandler(s.fixture, s.recorder))

	s.http = httptest.NewServer(mux)
	return s
}

// URL returns the Server base URL for BaseURL config or plain HTTP clients.
func (s *Server) URL() string { return s.http.URL }

// Host returns the host:port of URL for acceptanceharness.NewGuardedTransport.
func (s *Server) Host() string {
	u, err := url.Parse(s.http.URL)
	if err != nil {
		return ""
	}
	return u.Host
}

// Close releases the Server listener.
func (s *Server) Close() { s.http.Close() }

// Recorder returns the request recorder for sequence/auth assertions.
func (s *Server) Recorder() *acceptanceharness.Recorder { return s.recorder }

// Fixture returns the Fixture passed to NewServer.
func (s *Server) Fixture() *Fixture { return s.fixture }
