package githubingest

import (
	"strings"
	"testing"
)

func body_readerInternalTest_defaultsToGithubComWhenBaseURLIsUnset_64(t *testing.T, key []byte) {
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
}

func body_readerInternalTest_targetsTheConfiguredGitHubEnterpriseBaseURL_80(t *testing.T, key []byte) {
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
}
