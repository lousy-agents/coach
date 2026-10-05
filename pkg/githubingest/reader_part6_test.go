package githubingest_test

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// givenEnterpriseReader builds a reader configured with baseURL as its
// GitHub Enterprise BaseURL, wired to a transport that records every
// outbound request URL for inspection.
func givenEnterpriseReader(t *testing.T, baseURL string) (*githubingest.GitHubFileReader, *urlRecordingEnterpriseTransport) {
	t.Helper()

	transport := &urlRecordingEnterpriseTransport{}
	reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
		AppID:          1,
		InstallationID: 2,
		PrivateKey:     generateTestRSAPrivateKeyPEM(t),
		BaseURL:        baseURL,
		Transport:      transport,
	})
	if err != nil {
		t.Fatalf("NewGitHubFileReader: unexpected error: %v", err)
	}
	return reader, transport
}
