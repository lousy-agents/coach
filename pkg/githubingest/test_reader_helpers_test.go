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

// thenErrorIs fails the test unless errors.Is(err, target) holds.
func thenErrorIs(t *testing.T, err, target error, why string) {
	t.Helper()
	if !errors.Is(err, target) {
		t.Fatalf("%s: got err %v, want errors.Is(err, %v) to hold", why, err, target)
	}
}
