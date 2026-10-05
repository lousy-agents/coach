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

	"net/http"
	"strings"
	"testing"
)

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

func (f *fakeGitHubTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.HasSuffix(req.URL.Path, "/access_tokens") {
		return mintInstallationTokenResponse(req), nil
	}
	return f.handleContents(req), nil
}
