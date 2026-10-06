package fakegithub_test

import (
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v92/github"
	"github.com/lousy-agents/coach/internal/fakegithub"
	. "github.com/onsi/gomega"
)

// newAppsClient builds an App-JWT go-github client (ghinstallation AppsTransport).
func newAppsClient(server *fakegithub.Server) *github.Client {
	atr, err := ghinstallation.NewAppsTransport(http.DefaultTransport, contractAppID, contractRSAKey())
	Expect(err).NotTo(HaveOccurred())

	client, err := github.NewClient(
		github.WithEnterpriseURLs(server.URL(), server.URL()),
		github.WithTransport(atr),
	)
	Expect(err).NotTo(HaveOccurred())
	// Match githubingest: mint + API share BaseURL host/path.
	atr.BaseURL = client.BaseURL()
	return client
}
