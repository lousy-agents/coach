package githubingest_test

import (
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// givenReaderWithTokenMintFailing builds a reader whose transport fails the
// ghinstallation token-mint request itself with status, before go-github
// ever gets a chance to make the real Contents API call -- the transport
// panics if it sees a contents request, since one reaching it would mean
// ReadFile failed to short-circuit on the token-mint failure.
func givenReaderWithTokenMintFailing(t *testing.T, status int) *githubingest.GitHubFileReader {
	t.Helper()

	reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
		AppID:          1,
		InstallationID: 2,
		PrivateKey:     generateTestRSAPrivateKeyPEM(t),
		Transport:      &tokenMintFailureTransport{status: status},
	})
	if err != nil {
		t.Fatalf("NewGitHubFileReader: unexpected error: %v", err)
	}
	return reader
}
