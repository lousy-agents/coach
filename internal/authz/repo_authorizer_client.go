package authz

import (
	"net/http"

	"github.com/google/go-github/v92/github"
)

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
