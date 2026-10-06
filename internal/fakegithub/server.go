package fakegithub

import (
	"net/http/httptest"
	"net/url"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
)

// Server is an in-process httptest.Server for the OAuth, installation, and
// contents routes this package implements. It is driven by a caller-supplied
// [Fixture] and records every handled request via acceptanceharness.Recorder.
//
// Route paths live in routes.go; handler bodies live in the file named for
// their API (oauth.go, oauth_user.go, installation.go, permission.go,
// contents.go, repo_meta.go).
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
