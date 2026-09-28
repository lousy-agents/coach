package githubingest

import (
	"fmt"

	"net/http"

	"github.com/google/go-github/v92/github"
)

// mapContentsAPIError maps a failure from a GetContents-style call to the
// sentinel it represents:
//   - resp != nil (a genuine Contents API response): 404 -> ErrNotFound,
//     401/403 -> ErrAuth.
//   - resp == nil and the failure is a token-mint failure (see
//     isTokenMintFailure): always ErrAuth, regardless of the token-mint
//     endpoint's own status code. Minting a token is itself an
//     authentication step, so even a 404 there (e.g. an unknown or revoked
//     InstallationID) means "this GitHub App installation could not be
//     authenticated," never "this file doesn't exist" -- AC-5.5's
//     404 -> ErrNotFound scope is the Contents API's own response, not the
//     token-mint endpoint's.
//   - anything else: err wrapped with action for context, matching no
//     sentinel.
func mapContentsAPIError(err error, resp *github.Response, action string) error {
	if resp != nil {
		switch resp.StatusCode {
		case http.StatusNotFound:
			return fmt.Errorf("githubingest: %s: %w", action, ErrNotFound)
		case http.StatusUnauthorized, http.StatusForbidden:
			return fmt.Errorf("githubingest: %s rejected with status %d: %w", action, resp.StatusCode, ErrAuth)
		}
	} else if isTokenMintFailure(err) {
		return fmt.Errorf("githubingest: authenticating GitHub App installation while %s: %w", action, ErrAuth)
	}
	return fmt.Errorf("githubingest: %s: %w", action, err)
}

// NewGitHubFileReaderFromToken builds a GitHubFileReader authenticated with a
// pre-minted installation access token (from CredentialResolver.InstallationToken).
// baseURL is optional (GitHub Enterprise). Prefer this over NewGitHubFileReader
// when the caller already holds a token via the ADR-002 CredentialResolver seam.
func NewGitHubFileReaderFromToken(token, baseURL string) (*GitHubFileReader, error) {
	if token == "" {
		return nil, fmt.Errorf("githubingest: installation access token must be set")
	}
	opts := []github.ClientOptionsFunc{
		github.WithTimeout(DefaultGitHubFileReaderHTTPTimeout),
		github.WithAuthToken(token),
	}
	if baseURL != "" {
		opts = append(opts, github.WithEnterpriseURLs(baseURL, baseURL))
	}
	client, err := github.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("githubingest: building GitHub client from token: %w", err)
	}
	return &GitHubFileReader{client: client}, nil
}
