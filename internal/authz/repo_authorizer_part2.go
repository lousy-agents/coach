package authz

import (
	"errors"
)

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
