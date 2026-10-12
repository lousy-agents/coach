package githubingest_test

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// AC-5.2: constructor validation - an incomplete GitHubAppConfig (missing
// AppID, InstallationID, or PrivateKey) must be rejected before any client
// is built.
func TestNewGitHubFileReader_RejectsIncompleteConfig(t *testing.T) {
	validKey := generateTestRSAPrivateKeyPEM(t)

	tests := map[string]githubingest.GitHubAppConfig{
		"missing AppID": {
			InstallationID: 67890,
			PrivateKey:     validKey,
		},
		"missing InstallationID": {
			AppID:      12345,
			PrivateKey: validKey,
		},
		"missing PrivateKey": {
			AppID:          12345,
			InstallationID: 67890,
		},
		// Regression guard raised by review: negative IDs are not valid
		// GitHub App identifiers and must be rejected alongside zero/missing
		// ones, rather than producing a reader with impossible configuration
		// that only fails later during auth.
		"negative AppID": {
			AppID:          -1,
			InstallationID: 67890,
			PrivateKey:     validKey,
		},
		"negative InstallationID": {
			AppID:          12345,
			InstallationID: -1,
			PrivateKey:     validKey,
		},
	}

	for name, cfg := range tests {
		t.Run(name, func(t *testing.T) {
			expectIncompleteConfigRejected(t, cfg)
		})
	}
}

func expectIncompleteConfigRejected(t *testing.T, cfg githubingest.GitHubAppConfig) {
	reader, err := githubingest.NewGitHubFileReader(cfg)
	if err == nil {
		t.Fatalf("NewGitHubFileReader(%+v): got nil error, want error for incomplete config", cfg)
	}
	if reader != nil {
		t.Fatalf("NewGitHubFileReader(%+v): got non-nil reader %v alongside error, want nil reader", cfg, reader)
	}
}

// AC-5.2: NewGitHubFileReader builds an authenticated client using
// ghinstallation/v2 wrapping the configured base http.RoundTripper, given a
// complete GitHubAppConfig. No network access occurs during construction.
func TestNewGitHubFileReader_BuildsAuthenticatedClientFromConfig(t *testing.T) {
	cfg := githubingest.GitHubAppConfig{
		AppID:          12345,
		InstallationID: 67890,
		PrivateKey:     generateTestRSAPrivateKeyPEM(t),
	}

	reader, err := githubingest.NewGitHubFileReader(cfg)
	if err != nil {
		t.Fatalf("NewGitHubFileReader with complete config: unexpected error: %v", err)
	}
	if reader == nil {
		t.Fatalf("NewGitHubFileReader with complete config: got nil reader, want non-nil")
	}
}
