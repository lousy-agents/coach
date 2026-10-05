package fakegithub_test

import (
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v92/github"
	"github.com/lousy-agents/coach/internal/fakegithub"
	. "github.com/onsi/gomega"
)

func newInstallationClient(server *fakegithub.Server) *github.Client {
	itr, err := ghinstallation.New(http.DefaultTransport, contractAppID, contractInstallationID, contractRSAKey())
	Expect(err).NotTo(HaveOccurred())

	client, err := github.NewClient(
		github.WithEnterpriseURLs(server.URL(), server.URL()),
		github.WithTransport(itr),
	)
	Expect(err).NotTo(HaveOccurred())
	itr.BaseURL = client.BaseURL()
	return client
}
