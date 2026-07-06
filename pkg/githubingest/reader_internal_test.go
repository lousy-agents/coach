package githubingest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
)

// internalTestRSAPrivateKeyPEM mirrors the external test helper but lives in
// the internal test package (this file uses `package githubingest`, not
// `githubingest_test`, because it needs to inspect the unexported client
// field to verify AC-5.3's base URL wiring).
func internalTestRSAPrivateKeyPEM(t *testing.T) []byte {
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

// AC-5.3: where GitHubAppConfig.BaseURL is set, the reader shall target that
// GitHub Enterprise base URL instead of github.com.
func TestNewGitHubFileReader_TargetsConfiguredBaseURL(t *testing.T) {
	key := internalTestRSAPrivateKeyPEM(t)

	t.Run("defaults to github.com when BaseURL is unset", func(t *testing.T) {
		reader, err := NewGitHubFileReader(GitHubAppConfig{
			AppID:          1,
			InstallationID: 2,
			PrivateKey:     key,
		})
		if err != nil {
			t.Fatalf("NewGitHubFileReader: unexpected error: %v", err)
		}

		got := reader.client.BaseURL()
		if !strings.Contains(got, "api.github.com") {
			t.Fatalf("client base URL with no BaseURL configured: got %q, want it to target api.github.com", got)
		}
	})

	t.Run("targets the configured GitHub Enterprise base URL", func(t *testing.T) {
		const enterpriseURL = "https://ghe.example.com/"

		reader, err := NewGitHubFileReader(GitHubAppConfig{
			AppID:          1,
			InstallationID: 2,
			PrivateKey:     key,
			BaseURL:        enterpriseURL,
		})
		if err != nil {
			t.Fatalf("NewGitHubFileReader: unexpected error: %v", err)
		}

		got := reader.client.BaseURL()
		if !strings.Contains(got, "ghe.example.com") {
			t.Fatalf("client base URL with BaseURL=%q: got %q, want it to target ghe.example.com instead of github.com", enterpriseURL, got)
		}
		if strings.Contains(got, "api.github.com") {
			t.Fatalf("client base URL with BaseURL=%q: got %q, want it not to target api.github.com", enterpriseURL, got)
		}
	})
}
