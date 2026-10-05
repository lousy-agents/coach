// AC-5.10: every test in this package runs fully offline. Network access
// and real GitHub credentials are replaced throughout by
// generateTestRSAPrivateKeyPEM (a locally generated RSA key) and
// fakeGitHubTransport (a canned http.RoundTripper standing in for the
// GitHub API and the ghinstallation token-mint endpoint).
package githubingest_test

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// newTestReader builds a GitHubFileReader wired to an offline fake
// transport: the ghinstallation token mint is answered automatically, and
// handleContents answers the Contents API call under test.
func newTestReader(t *testing.T, handleContents contentsHandlerFunc) *githubingest.GitHubFileReader {
	t.Helper()

	reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
		AppID:          12345,
		InstallationID: 67890,
		PrivateKey:     generateTestRSAPrivateKeyPEM(t),
		Transport:      &fakeGitHubTransport{handleContents: handleContents},
	})
	if err != nil {
		t.Fatalf("newTestReader: NewGitHubFileReader failed: %v", err)
	}
	return reader
}
