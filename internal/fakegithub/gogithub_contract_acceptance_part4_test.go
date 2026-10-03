package fakegithub_test

import (
	"github.com/google/go-github/v92/github"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/fakegithub"
)

func newOAuthClient(server *fakegithub.Server, token string) *github.Client {
	client, err := github.NewClient(
		github.WithEnterpriseURLs(server.URL(), server.URL()),
		github.WithAuthToken(token),
	)
	Expect(err).NotTo(HaveOccurred())
	return client
}
