package acceptanceharness

import (
	"net/http"
)

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
