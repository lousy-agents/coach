package authn

import (
	"net/http"
	"testing"
	"time"
)

func TestNew_DefaultGitHubHTTPClientHasTimeout(t *testing.T) {
	svc, err := New(Options{
		SigningKey: []byte("test-signing-secret-at-least-32-bytes!!"),
		Issuer:     "https://coach.test",
		TokenTTL:   time.Hour,
		GitHubOAuth: &GitHubOAuthConfig{
			ClientID:     "id",
			ClientSecret: "secret",
			BaseURL:      "https://github.example",
			RedirectURI:  "https://coach.test/oauth/github/callback",
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if svc.httpClient == nil {
		t.Fatal("httpClient must be set when GitHubOAuth is configured")
	}
	if svc.httpClient == http.DefaultClient {
		t.Fatal("default GitHub HTTP client must not be http.DefaultClient (no timeout)")
	}
	if got, want := svc.httpClient.Timeout, DefaultGitHubHTTPClientTimeout; got != want {
		t.Fatalf("default GitHub HTTP client Timeout: got %v want %v", got, want)
	}
}

func TestNew_RejectsNonAbsoluteGitHubBaseURL(t *testing.T) {
	_, err := New(Options{
		SigningKey: []byte("test-signing-secret-at-least-32-bytes!!"),
		Issuer:     "https://coach.test",
		GitHubOAuth: &GitHubOAuthConfig{
			ClientID:     "id",
			ClientSecret: "secret",
			BaseURL:      "not-a-url",
			RedirectURI:  "https://coach.test/oauth/github/callback",
		},
	})
	if err == nil {
		t.Fatal("New must reject non-absolute GitHubOAuth.BaseURL")
	}
}
