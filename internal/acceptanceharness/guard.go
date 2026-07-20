// Package acceptanceharness provides the offline acceptance-test guard
// rails for Feature Zero (GitHub issue #76, epic #73): an ambient-credential
// scanner that flags leaked-in, CI-style GitHub/AWS credentials before an
// acceptance suite runs, and a no-egress http.RoundTripper that makes an
// accidental public network call (e.g. to api.github.com) observable and
// failing rather than merely discouraged.
package acceptanceharness

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
)

// AmbientCredentialVars lists the environment variable names that, if
// present in a process's environment, indicate ambient GitHub or AWS
// credentials that an offline acceptance suite must not rely on. The last
// two AWS entries are the ECS/instance-metadata credential-source
// indicators, not literal credential values, but their presence still
// signals that ambient credentials would be resolvable.
var AmbientCredentialVars = []string{
	"GITHUB_TOKEN",
	"GH_TOKEN",
	"GITHUB_APP_ID",
	"GITHUB_APP_PRIVATE_KEY",
	"GITHUB_APP_INSTALLATION_ID",
	"AWS_ACCESS_KEY_ID",
	"AWS_SECRET_ACCESS_KEY",
	"AWS_SESSION_TOKEN",
	"AWS_PROFILE",
	"AWS_SHARED_CREDENTIALS_FILE",
	"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI",
	"AWS_CONTAINER_CREDENTIALS_FULL_URI",
}

// CredentialGuardResult records the ambient-credential guard's decision:
// which of AmbientCredentialVars were found present in a scanned
// environment.
type CredentialGuardResult struct {
	// Found holds the names, in AmbientCredentialVars order, of every
	// ambient-credential variable found present.
	Found []string
}

// Rejected reports whether the scan found any ambient-credential variable
// present.
func (r CredentialGuardResult) Rejected() bool {
	return len(r.Found) > 0
}

// ScanEnviron scans environ (shaped like os.Environ(): "KEY=VALUE" entries)
// for any of AmbientCredentialVars and returns which were found present. It
// is a pure function that never touches the real process environment, so
// it is testable without mutating global state.
func ScanEnviron(environ []string) CredentialGuardResult {
	present := make(map[string]bool, len(environ))
	for _, kv := range environ {
		name, _, ok := strings.Cut(kv, "=")
		if !ok {
			continue
		}
		present[name] = true
	}

	var found []string
	for _, name := range AmbientCredentialVars {
		if present[name] {
			found = append(found, name)
		}
	}
	return CredentialGuardResult{Found: found}
}

// ScanProcessEnv scans the real process environment (os.Environ()) for
// ambient-credential variables.
func ScanProcessEnv() CredentialGuardResult {
	return ScanEnviron(os.Environ())
}

// ScrubProcessEnv unsets any ambient-credential variable present in the
// real process environment and returns the names it scrubbed, in
// AmbientCredentialVars order.
func ScrubProcessEnv() []string {
	result := ScanProcessEnv()
	for _, name := range result.Found {
		os.Unsetenv(name)
	}
	return result.Found
}

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

// BlockedRequests returns the full URL (as a string) of every request this
// transport refused because its host was not on the allowlist, in the
// order they were attempted, so a test can assert that an accidental
// public request (e.g. to https://api.github.com/...) was observed and
// blocked.
func (g *GuardedTransport) BlockedRequests() []string {
	g.mu.Lock()
	defer g.mu.Unlock()

	out := make([]string, len(g.blocked))
	copy(out, g.blocked)
	return out
}

// RoundTrip rejects any request whose host is not on the allowlist before
// ever attempting to send it -- no dial, real or otherwise, is attempted
// for a blocked host. Allowed requests are delegated to the injected fake
// transport.
func (g *GuardedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	host := req.URL.Host
	if !g.allowed[host] {
		g.mu.Lock()
		g.blocked = append(g.blocked, req.URL.String())
		g.mu.Unlock()
		return nil, fmt.Errorf("acceptanceharness: blocked disallowed egress to %s (host %q is not in the allowlist)", req.URL, host)
	}
	if g.fake == nil {
		return nil, fmt.Errorf("acceptanceharness: no fake transport configured for allowed host %q", host)
	}
	return g.fake.RoundTrip(req)
}
