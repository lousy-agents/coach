// Package acceptanceharness provides the offline acceptance-test guard
// rails for Feature Zero (GitHub issue #76, epic #73): an ambient-credential
// scanner that flags leaked-in, CI-style GitHub/AWS credentials before an
// acceptance suite runs, and a no-egress http.RoundTripper that makes an
// accidental public network call (e.g. to api.github.com) observable and
// failing rather than merely discouraged.
package acceptanceharness

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// AmbientCredentialVars lists the environment variable names that, if
// present in a process's environment, indicate ambient credentials that an
// offline acceptance suite must not rely on, across four categories: GitHub
// App (server-to-server), AWS, GitHub OAuth App (the Coach identity
// provider; see docs/architecture/ADR-001-coach-api-authentication.md), and
// model-provider API keys (see docs/architecture/system-overview.md's model
// gateway). The AWS_CONTAINER_CREDENTIALS_* entries are the ECS/
// instance-metadata credential-source indicators, not literal credential
// values, but their presence still signals that ambient credentials would be
// resolvable.
var AmbientCredentialVars = []string{
	// GitHub App.
	"GITHUB_TOKEN",
	"GH_TOKEN",
	"GITHUB_APP_ID",
	"GITHUB_APP_PRIVATE_KEY",
	"GITHUB_APP_INSTALLATION_ID",
	// AWS.
	"AWS_ACCESS_KEY_ID",
	"AWS_SECRET_ACCESS_KEY",
	"AWS_SESSION_TOKEN",
	"AWS_PROFILE",
	"AWS_SHARED_CREDENTIALS_FILE",
	"AWS_CONFIG_FILE",
	"AWS_CONTAINER_CREDENTIALS_RELATIVE_URI",
	"AWS_CONTAINER_CREDENTIALS_FULL_URI",
	// GitHub OAuth (Coach identity provider).
	"GITHUB_CLIENT_ID",
	"GITHUB_CLIENT_SECRET",
	// Model provider.
	"OPENAI_API_KEY",
	"ANTHROPIC_API_KEY",
}

// AmbientCredentialFiles lists default ambient-credential file locations,
// relative to a home directory, that indicate resolvable ambient
// credentials even when no environment variable points at them (e.g. the
// AWS SDK's default shared-credentials file, consulted regardless of
// AWS_SHARED_CREDENTIALS_FILE/AWS_PROFILE being set). This also includes the
// AWS SDK's default shared-config file (~/.aws/config): the default
// provider chain reads it too, and it can itself hold static access keys
// (e.g. under a profile's aws_access_key_id/aws_secret_access_key), not just
// non-secret settings like region -- so an acceptance process with
// credentials only there must still be rejected.
//
// Known, deferred limitation: this package does not probe the cloud
// instance-metadata endpoint (e.g. 169.254.169.254) as an ambient-credential
// source, because doing so would itself require a network call -- in
// tension with the no-egress principle this guard exists to enforce. See
// docs/architecture/acceptance-harness.md section 4 for the accepted
// rationale; later Feature Zero tasks (0.3's Compose no-egress topology)
// are what actually prevent metadata-endpoint reachability at runtime, not
// this guard.
var AmbientCredentialFiles = []string{
	filepath.Join(".aws", "credentials"),
	filepath.Join(".aws", "config"),
}

// CredentialGuardResult records the ambient-credential guard's decision:
// which of AmbientCredentialVars were found present in a scanned
// environment, and which of AmbientCredentialFiles were found present on
// disk.
type CredentialGuardResult struct {
	// Found holds the names, in AmbientCredentialVars order, of every
	// ambient-credential variable found present.
	Found []string
	// FoundFiles holds the paths, in AmbientCredentialFiles order, of every
	// default ambient-credential file found present on disk.
	FoundFiles []string
}

// Rejected reports whether the scan found any ambient-credential variable
// or default ambient-credential file present.
func (r CredentialGuardResult) Rejected() bool {
	return len(r.Found) > 0 || len(r.FoundFiles) > 0
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

// ScanCredentialFiles joins home with each AmbientCredentialFiles entry and
// returns the ones for which exists(path) is true. It is a pure function
// that never touches the real filesystem itself -- the caller-supplied
// exists func is the only thing that does -- so it is testable without
// depending on (or risking mutation of) a real home directory's actual
// credential files.
func ScanCredentialFiles(home string, exists func(path string) bool) []string {
	var found []string
	for _, rel := range AmbientCredentialFiles {
		path := filepath.Join(home, rel)
		if exists(path) {
			found = append(found, path)
		}
	}
	return found
}

// ScanProcessEnv scans the real process environment (os.Environ()) for
// ambient-credential variables, and the real home directory for default
// ambient-credential files (AmbientCredentialFiles). If the home directory
// cannot be resolved, the file check is skipped (Found still reflects the
// environment-variable scan).
func ScanProcessEnv() CredentialGuardResult {
	result := ScanEnviron(os.Environ())

	home, err := os.UserHomeDir()
	if err != nil {
		return result
	}
	result.FoundFiles = ScanCredentialFiles(home, func(path string) bool {
		_, statErr := os.Stat(path)
		return statErr == nil
	})
	return result
}

// RejectAmbientCredentials is the guard's activation entry point: it scans
// the real process environment and default ambient-credential file
// locations (via ScanProcessEnv), and if anything is found, writes a clear
// diagnostic to w listing every violation (env var names and/or file paths
// found) and returns false. If nothing is found, it writes nothing and
// returns true.
//
// This is reject-only: it never scrubs or otherwise mutates the process
// environment or filesystem. A caller must itself fail its run (e.g.
// os.Exit(1)) when this returns false.
func RejectAmbientCredentials(w io.Writer) bool {
	result := ScanProcessEnv()
	if !result.Rejected() {
		return true
	}

	fmt.Fprintln(w, "acceptanceharness: refusing to run -- ambient credentials detected:")
	for _, name := range result.Found {
		fmt.Fprintf(w, "  - environment variable %s is set\n", name)
	}
	for _, path := range result.FoundFiles {
		fmt.Fprintf(w, "  - credential file %s exists\n", path)
	}
	fmt.Fprintln(w, "unset these variables and/or remove or relocate these files before running acceptance suites offline.")
	return false
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
