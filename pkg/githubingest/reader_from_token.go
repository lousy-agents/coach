package githubingest

import (
	"fmt"

	"github.com/google/go-github/v92/github"
)

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
