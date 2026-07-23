package githubingest

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v89/github"
)

// DefaultCredentialResolverHTTPTimeout bounds outbound App-JWT-authenticated
// HTTP calls (installation resolution, token minting) issued by a
// CredentialResolver.
const DefaultCredentialResolverHTTPTimeout = 10 * time.Second

// CredentialResolver mints GitHub App installation tokens: the single seam
// ADR-002 rule 5 requires, shared by GitHubFileReader-style fixed-installation
// reads and internal/authz's dynamic owner/repo -> installation resolution.
// It never appears without a valid AppID/PrivateKey; it holds no long-lived
// installation token itself (a fresh mint per InstallationToken call, no
// caching -- matches ADR-003's "no caching in v1").
type CredentialResolver struct {
	client *github.Client
}

// CredentialResolverConfig configures a CredentialResolver's GitHub App
// authentication.
type CredentialResolverConfig struct {
	AppID      int64
	PrivateKey []byte            // PEM (PKCS#1), never logged
	BaseURL    string            // optional; GitHub Enterprise
	Transport  http.RoundTripper // optional; base transport (tests)
}

// NewCredentialResolver builds a CredentialResolver authenticated as the
// GitHub App described by cfg.
func NewCredentialResolver(cfg CredentialResolverConfig) (*CredentialResolver, error) {
	if cfg.AppID <= 0 {
		return nil, fmt.Errorf("githubingest: CredentialResolverConfig.AppID must be a positive ID, got %d", cfg.AppID)
	}
	if len(cfg.PrivateKey) == 0 {
		return nil, fmt.Errorf("githubingest: CredentialResolverConfig.PrivateKey must be set")
	}

	base := cfg.Transport
	if base == nil {
		base = http.DefaultTransport
	}

	atr, err := ghinstallation.NewAppsTransport(base, cfg.AppID, cfg.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("githubingest: building App JWT transport: %w", err)
	}

	opts := []github.ClientOptionsFunc{github.WithTransport(atr), github.WithTimeout(DefaultCredentialResolverHTTPTimeout)}
	if cfg.BaseURL != "" {
		opts = append(opts, github.WithEnterpriseURLs(cfg.BaseURL, cfg.BaseURL))
	}

	client, err := github.NewClient(opts...)
	if err != nil {
		return nil, fmt.Errorf("githubingest: building GitHub client: %w", err)
	}

	if cfg.BaseURL != "" {
		// See GitHubFileReader's NewGitHubFileReader: go-github's normalized
		// API base and ghinstallation's Transport.BaseURL diverge for a bare
		// Enterprise host, so hand ghinstallation go-github's own normalized
		// value to keep both on the same host and path prefix.
		atr.BaseURL = client.BaseURL()
	}

	return &CredentialResolver{client: client}, nil
}

// ResolveInstallationID resolves the installation governing owner/repo via
// GET /repos/{owner}/{repo}/installation (App JWT auth). Returns ErrNotFound
// on GitHub 404 (deliberately covers both "no such repo" and "App not
// installed there" -- ADR-003) and ErrAuth on 401/403. Any other failure is
// returned wrapped, unmatched to a sentinel (caller treats as transient/503).
func (c *CredentialResolver) ResolveInstallationID(ctx context.Context, owner, repo string) (int64, error) {
	installation, resp, err := c.client.Apps.GetRepositoryInstallation(ctx, owner, repo)
	if err != nil {
		return 0, mapCredentialAPIError(err, resp, fmt.Sprintf("resolving installation for %s/%s", owner, repo))
	}
	return installation.GetID(), nil
}

// InstallationToken mints a fresh installation access token scoped to
// installationID via POST /app/installations/{id}/access_tokens. Maps errors
// the same way as ResolveInstallationID.
func (c *CredentialResolver) InstallationToken(ctx context.Context, installationID int64) (string, error) {
	token, resp, err := c.client.Apps.CreateInstallationToken(ctx, installationID, nil)
	if err != nil {
		return "", mapCredentialAPIError(err, resp, fmt.Sprintf("minting installation token for installation %d", installationID))
	}
	return token.GetToken(), nil
}

// mapCredentialAPIError maps a failed App-JWT-authenticated call to the
// sentinel it represents, mirroring mapContentsAPIError's status mapping.
func mapCredentialAPIError(err error, resp *github.Response, action string) error {
	if resp != nil {
		switch resp.StatusCode {
		case http.StatusNotFound:
			return fmt.Errorf("githubingest: %s: %w", action, ErrNotFound)
		case http.StatusUnauthorized, http.StatusForbidden:
			return fmt.Errorf("githubingest: %s rejected with status %d: %w", action, resp.StatusCode, ErrAuth)
		}
	}
	return fmt.Errorf("githubingest: %s: %w", action, err)
}
