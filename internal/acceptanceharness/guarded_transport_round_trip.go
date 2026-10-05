package acceptanceharness

import (
	"fmt"
	"net/http"
)

// RoundTrip rejects any request whose host is not on the allowlist before
// ever attempting to send it -- no dial, real or otherwise, is attempted
// for a blocked host. Allowed requests are delegated to the injected fake
// transport.
func (g *GuardedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Host
	if !g.allowed[host] {
		scrubbed := scrubURL(req.URL)
		g.mu.Lock()
		g.blocked = append(g.blocked, scrubbed)
		g.mu.Unlock()
		return nil, fmt.Errorf("acceptanceharness: blocked disallowed egress to %s (host %q is not in the allowlist)", scrubbed, host)
	}
	if g.fake == nil {
		return nil, fmt.Errorf("acceptanceharness: no fake transport configured for allowed host %q", host)
	}
	return g.fake.RoundTrip(req)
}
