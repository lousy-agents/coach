package githubingest_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// AC-5.6 regression: when the ghinstallation token-mint request itself fails
// with 401/403 (e.g. a revoked or misconfigured GitHub App installation),
// go-github never issues the actual Contents API request, so ReadFile's
// error has no *http.Response of its own to inspect -- the failure is
// wrapped inside a *ghinstallation.HTTPError in the error chain instead.
// Previously this surfaced as a generic wrapped error rather than ErrAuth.
func TestReadFile_TokenMintAuthFailureReturnsErrAuth(t *testing.T) {
	statuses := map[string]int{
		"401 Unauthorized": http.StatusUnauthorized,
		"403 Forbidden":    http.StatusForbidden,

		"404 Not Found (wrong InstallationID)": http.StatusNotFound,
	}

	for name, status := range statuses {
		t.Run(name, func(t *testing.T) {
			reader := givenReaderWithTokenMintFailing(t, status)
			ref := githubingest.GitHubFileRef{Owner: "acme", Repo: "widgets", Ref: "main", Path: "secret.txt"}

			_, _, err := reader.ReadFile(context.Background(), ref)

			thenErrorIs(t, err, githubingest.ErrAuth, fmt.Sprintf("ReadFile with token mint failing %d", status))
		})
	}
}

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
