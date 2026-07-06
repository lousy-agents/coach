// AC-5.10: every test in this package runs fully offline. Network access
// and real GitHub credentials are replaced throughout by
// generateTestRSAPrivateKeyPEM (a locally generated RSA key) and
// fakeGitHubTransport (a canned http.RoundTripper standing in for the
// GitHub API and the ghinstallation token-mint endpoint).
package githubingest_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// thenErrorIs fails the test unless errors.Is(err, target) holds.
func thenErrorIs(t *testing.T, err, target error, why string) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("%s: got err %v, want errors.Is(err, %v) to hold", why, err, target)
	}
}

// generateTestRSAPrivateKeyPEM returns a freshly generated RSA private key,
// PKCS#1-PEM-encoded the same way GitHub issues App private keys. It never
// touches the network or any real credentials.
func generateTestRSAPrivateKeyPEM(t *testing.T) []byte {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating test RSA key: %v", err)
	}

	block := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}

	return pem.EncodeToMemory(block)
}

// contentsHandlerFunc builds the canned *http.Response for a Contents API
// call in a fake transport. It never touches the network.
type contentsHandlerFunc func(req *http.Request) *http.Response

// fakeGitHubTransport is an offline http.RoundTripper stand-in for GitHub's
// API. It answers the ghinstallation installation-token mint request with a
// canned token, and delegates every Contents API call -- both the direct
// file request and the parent-directory listing ReadFile uses to detect
// symlinks (AC-5.7) -- to handleContents, so a test can distinguish them by
// inspecting req.URL.Path when it needs to.
type fakeGitHubTransport struct {
	handleContents contentsHandlerFunc
}

func (f *fakeGitHubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.HasSuffix(req.URL.Path, "/access_tokens") {
		return mintInstallationTokenResponse(req), nil
	}
	return f.handleContents(req), nil
}

// mintInstallationTokenResponse answers ghinstallation's installation access
// token exchange with a canned, never-expiring (for the test's short
// lifetime) token. No network access occurs.
func mintInstallationTokenResponse(req *http.Request) *http.Response {
	const body = `{"token":"test-installation-token","expires_at":"2999-01-01T00:00:00Z"}`
	return &http.Response{
		Status:     "201 Created",
		StatusCode: http.StatusCreated,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    req,
	}
}

// jsonResponse builds a canned *http.Response carrying body as its JSON
// payload with the given status code.
func jsonResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		Status:     http.StatusText(status),
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    req,
	}
}

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
