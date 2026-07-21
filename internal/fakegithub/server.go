package fakegithub

import (
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// Server is the Coach-owned fake GitHub HTTP service: an in-process
// httptest.Server answering the OAuth, GitHub App installation/
// authorization, and repository-content-read routes fakegithub implements,
// driven entirely by a caller-supplied Fixture and recording every request
// it handles via acceptanceharness.Recorder.
//
// Route wiring lives here and is meant to stay stable: later work replaces
// the handler bodies in oauth.go/installation.go/contents.go without
// needing to touch this file.
type Server struct {
	http     *httptest.Server
	fixture  *Fixture
	recorder *acceptanceharness.Recorder
}

// NewServer starts a Server backed by fixture, which must outlive the
// Server (the Server does not copy it). Callers must Close the returned
// Server when done.
func NewServer(fixture *Fixture) *Server {
	if fixture == nil {
		panic("fakegithub: NewServer called with a nil Fixture")
	}

	s := &Server{fixture: fixture, recorder: &acceptanceharness.Recorder{}}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /login/oauth/authorize", oauthAuthorizeHandler(s.fixture, s.recorder))
	mux.HandleFunc("POST /login/oauth/access_token", oauthTokenHandler(s.fixture, s.recorder))
	mux.HandleFunc("GET /user", oauthUserHandler(s.fixture, s.recorder))

	// go-github's client, pointed at a bare-host BaseURL via
	// github.WithEnterpriseURLs, prefixes every repos/app API request with
	// "api/v3/" (see pkg/githubingest.NewGitHubFileReader's comment on
	// itr.BaseURL = client.BaseURL()). "/user" and "/login/oauth/..." are
	// never touched by go-github in this codebase, so they stay at the
	// bare path, matching real github.com's shape.
	mux.HandleFunc("POST /api/v3/app/installations/{id}/access_tokens", installationTokenHandler(s.fixture, s.recorder))
	mux.HandleFunc("GET /api/v3/repos/{owner}/{repo}/installation", installationResolutionHandler(s.fixture, s.recorder))
	mux.HandleFunc("GET /api/v3/repos/{owner}/{repo}/collaborators/{username}/permission", permissionHandler(s.fixture, s.recorder))
	mux.HandleFunc("GET /api/v3/repos/{owner}/{repo}/contents/{path...}", contentsHandler(s.fixture, s.recorder))

	s.http = httptest.NewServer(mux)
	return s
}

// URL returns the Server's base URL (e.g. "http://127.0.0.1:54321"),
// suitable for pkg/githubingest.GitHubAppConfig.BaseURL or a plain
// net/http.Client request against /user or /login/oauth/....
func (s *Server) URL() string { return s.http.URL }

// Host returns the host:port portion of URL(), suitable for
// acceptanceharness.NewGuardedTransport's allowedHosts.
func (s *Server) Host() string {
	u, err := url.Parse(s.http.URL)
	if err != nil {
		return ""
	}
	return u.Host
}

// Close shuts down the Server, releasing its listener.
func (s *Server) Close() { s.http.Close() }

// Recorder returns the Server's request recorder, so a test can assert the
// exact sequence, fixture/scenario, and authentication mode of requests
// made against the Server.
func (s *Server) Recorder() *acceptanceharness.Recorder { return s.recorder }

// Fixture returns the Fixture the Server was constructed with.
func (s *Server) Fixture() *Fixture { return s.fixture }
