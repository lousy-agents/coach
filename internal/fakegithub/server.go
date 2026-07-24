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

// requireFixture panics with a caller-identifying message if fixture is nil,
// so NewServer and Handler share one guard without duplicating the panic
// message string.
func requireFixture(caller string, fixture *Fixture) {
	if fixture == nil {
		panic("fakegithub: " + caller + " called with a nil Fixture")
	}
}

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
	// Repo metadata + commits for githubingest.ResolveCommitSHA (default branch + ref → SHA).
	// Register commits before bare repo so the more-specific path wins if routers differ.
	mux.HandleFunc("GET /api/v3/repos/{owner}/{repo}/commits/{ref}", commitHandler(fixture, rec))
	mux.HandleFunc("GET /api/v3/repos/{owner}/{repo}", repoMetaHandler(fixture, rec))

	return mux, rec
}

// NewServer starts a Server backed by fixture. fixture must outlive the
// Server (it is not copied). Callers must Close when done. Panics if fixture
// is nil.
func NewServer(fixture *Fixture) *Server {
	requireFixture("NewServer", fixture)

	handler, rec := Handler(fixture)
	return &Server{fixture: fixture, recorder: rec, http: httptest.NewServer(handler)}
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
