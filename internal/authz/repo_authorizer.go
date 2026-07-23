package authz

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/go-github/v89/github"

	"github.com/lousy-agents/coach/pkg/githubingest"
)

// DefaultGitHubRepoAuthorizerHTTPTimeout bounds the collaborator-permission
// HTTP call GitHubRepoAuthorizer issues as the installation (step c of
// ADR-003's algorithm); installation resolution and token minting are bounded
// by the shared CredentialResolver's own timeout.
const DefaultGitHubRepoAuthorizerHTTPTimeout = 10 * time.Second

// GitHubRepoAuthorizerConfig configures a GitHubRepoAuthorizer.
type GitHubRepoAuthorizerConfig struct {
	// Credentials is the shared installation-token seam (ADR-002 rule 5).
	Credentials *githubingest.CredentialResolver
	BaseURL     string            // optional; GitHub Enterprise (must match Credentials' own host)
	Transport   http.RoundTripper // optional; base transport (tests)
}

// GitHubRepoAuthorizer implements ADR-003's repository authorization policy
// against the real (or fake) GitHub API.
type GitHubRepoAuthorizer struct {
	credentials *githubingest.CredentialResolver
	baseURL     string
	transport   http.RoundTripper
}

// NewGitHubRepoAuthorizer builds a GitHubRepoAuthorizer. cfg.Credentials is required.
func NewGitHubRepoAuthorizer(cfg GitHubRepoAuthorizerConfig) (*GitHubRepoAuthorizer, error) {
	if cfg.Credentials == nil {
		return nil, errors.New("authz: GitHubRepoAuthorizerConfig.Credentials is required")
	}
	return &GitHubRepoAuthorizer{
		credentials: cfg.Credentials,
		baseURL:     cfg.BaseURL,
		transport:   cfg.Transport,
	}, nil
}

// Authorize implements ADR-003's three-step algorithm: resolve the governing
// installation, mint an installation token, then check the principal's
// effective permission as that installation.
func (a *GitHubRepoAuthorizer) Authorize(ctx context.Context, login, owner, repo string) error {
	installationID, err := a.credentials.ResolveInstallationID(ctx, owner, repo)
	if err != nil {
		if errors.Is(err, githubingest.ErrNotFound) {
			// A nonexistent repo and an App-uninstalled repo are
			// indistinguishable at this step (ADR-003); both collapse to
			// ErrNotAuthorized.
			return fmt.Errorf("authz: resolving installation for %s/%s: %w", owner, repo, ErrNotAuthorized)
		}
		return fmt.Errorf("authz: resolving installation for %s/%s: %w", owner, repo, err)
	}

	token, err := a.credentials.InstallationToken(ctx, installationID)
	if err != nil {
		return fmt.Errorf("authz: minting installation token for %s/%s: %w", owner, repo, err)
	}

	client, err := a.permissionClient(token)
	if err != nil {
		return fmt.Errorf("authz: building permission-check client: %w", err)
	}

	level, resp, err := client.Repositories.GetPermissionLevel(ctx, owner, repo, login)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			// The principal literally has no relationship with the repo.
			return fmt.Errorf("authz: %s has no relationship with %s/%s: %w", login, owner, repo, ErrNotAuthorized)
		}
		return fmt.Errorf("authz: checking permission for %s on %s/%s: %w", login, owner, repo, err)
	}

	if level.GetPermission() == "none" {
		return fmt.Errorf("authz: %s has no role in %s/%s: %w", login, owner, repo, ErrNotAuthorized)
	}
	return nil
}

// permissionClient builds a go-github client authenticated with the freshly
// minted installation token, targeting the same host as a.credentials.
func (a *GitHubRepoAuthorizer) permissionClient(token string) (*github.Client, error) {
	transport := a.transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	opts := []github.ClientOptionsFunc{
		github.WithTransport(transport),
		github.WithTimeout(DefaultGitHubRepoAuthorizerHTTPTimeout),
		github.WithAuthToken(token),
	}
	if a.baseURL != "" {
		opts = append(opts, github.WithEnterpriseURLs(a.baseURL, a.baseURL))
	}
	return github.NewClient(opts...)
}
