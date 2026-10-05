package acceptanceharness

import (
	"net/http"
	"net/url"
	"sync"
)

// GuardedTransport is an http.RoundTripper that rejects any request whose
// host is not on an explicit allowlist before the request is ever sent,
// making an accidental public network call observable and failing rather
// than merely discouraged. Requests to allowed hosts are delegated to an
// injected fake http.RoundTripper for in-process fakes; GuardedTransport
// itself never dials a real network connection.
type GuardedTransport struct {
	allowed map[string]bool
	fake    http.RoundTripper

	mu      sync.Mutex
	blocked []string
}

// NewGuardedTransport builds a GuardedTransport permitting only requests
// whose URL host (as reported by (*url.URL).Host, e.g. "127.0.0.1:9999")
// appears in allowedHosts, delegating permitted requests to fake.
func NewGuardedTransport(allowedHosts []string, fake http.RoundTripper) *GuardedTransport {
	allowed := make(map[string]bool, len(allowedHosts))
	for _, host := range allowedHosts {
		allowed[host] = true
	}
	return &GuardedTransport{allowed: allowed, fake: fake}
}

// BlockedRequests returns a scrubbed, credential-free URL (as a string) of
// every request this transport refused because its host was not on the
// allowlist, in the order they were attempted, so a test can assert that an
// accidental public request (e.g. to https://api.github.com/...) was
// observed and blocked. Any userinfo, query string, or fragment embedded in
// the original URL is stripped before recording, so this diagnostic never
// leaks credentials accidentally embedded in a blocked URL; scheme, host,
// and path are preserved.
func (g *GuardedTransport) BlockedRequests() []string {
	g.mu.Lock()
	defer g.mu.Unlock()

	out := make([]string, len(g.blocked))
	copy(out, g.blocked)
	return out
}

// scrubURL returns a credential-free string form of u: userinfo, query
// string, and fragment are stripped, while scheme, host, and path are
// preserved for diagnostics. This guards against a caller accidentally
// embedding credentials in a blocked request's URL (e.g. userinfo or a
// query-string access token) leaking into recorded diagnostics or error
// messages.
func scrubURL(u *url.URL) string {
	scrubbed := *u
	scrubbed.User = nil
	scrubbed.RawQuery = ""
	scrubbed.Fragment = ""
	scrubbed.RawFragment = ""
	return scrubbed.String()
}
