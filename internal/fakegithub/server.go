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

// /user is github.com's public path (raw net/http tests).
// /api/v3/user is what go-github emits with WithEnterpriseURLs (api/v3 prefix).
// OAuth authorize/token stay on bare paths; App/repos APIs use api/v3.

// Register commits before bare repo so the more-specific path wins.

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

// Fixture returns the Fixture passed to NewServer.
