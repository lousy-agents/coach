package githubingest

import (
	"fmt"
	"net/http"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v92/github"
)

// DefaultGitHubFileReaderHTTPTimeout bounds outbound Contents API HTTP calls
// issued by GitHubFileReader (ReadFile and parent-directory listings). Matches
// DefaultCredentialResolverHTTPTimeout so App-authenticated paths share one
// production hang bound.
const DefaultGitHubFileReaderHTTPTimeout = 10 * time.Second

// GitHubAppConfig is GitHub App authentication for GitHubFileReader.
type GitHubAppConfig struct {
	AppID          int64
	InstallationID int64
	PrivateKey     []byte            // PEM (PKCS#1) as issued by GitHub; never logged
	BaseURL        string            // optional; GitHub Enterprise
	Transport      http.RoundTripper // optional; base transport (tests, future rate limiting)
}

// GitHubFileReader reads file contents from GitHub repositories,
// authenticated as a GitHub App installation.
type GitHubFileReader struct {
	client *github.Client
}

// NewGitHubFileReader builds a GitHubFileReader authenticated as the GitHub
// App installation described by cfg.
func NewGitHubFileReader(cfg GitHubAppConfig) (*GitHubFileReader, error) {
	if cfg.AppID <= 0 {
		return nil, fmt.Errorf("githubingest: GitHubAppConfig.AppID must be a positive ID, got %d", cfg.AppID)
	}
	if cfg.InstallationID <= 0 {
		return nil, fmt.Errorf("githubingest: GitHubAppConfig.InstallationID must be a positive ID, got %d", cfg.InstallationID)
	}
	if len(cfg.PrivateKey) == 0 {
		return nil, fmt.Errorf("githubingest: GitHubAppConfig.PrivateKey must be set")
	}

	base := cfg.Transport
	if base == nil {
		base = http.DefaultTransport
	}

	itr, err := ghinstallation.New(base, cfg.AppID, cfg.InstallationID, cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("githubingest: building installation transport: %w", err)
	}

	opts := []github.ClientOptionsFunc{
		github.WithTransport(itr),
		github.WithTimeout(DefaultGitHubFileReaderHTTPTimeout),
	}
	if cfg.BaseURL != "" {
		opts = append(opts, github.WithEnterpriseURLs(cfg.BaseURL, cfg.BaseURL))
	}

	client, err := github.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("githubingest: building GitHub client: %w", err)
	}

	if cfg.BaseURL != "" {
		// go-github's WithEnterpriseURLs and ghinstallation's Transport.BaseURL
		// normalize a bare Enterprise host differently (go-github appends
		// "api/v3/" to the path; ghinstallation does not). Read back
		// go-github's normalized API base and hand ghinstallation that exact
		// value, so the installation-token-mint request and the Contents API
		// request hit the same host and path prefix.
		itr.BaseURL = client.BaseURL()
	}

	return &GitHubFileReader{client: client}, nil
}
