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
	"os"
	"path/filepath"
	"strings"
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
