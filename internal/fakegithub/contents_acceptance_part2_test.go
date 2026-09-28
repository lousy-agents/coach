package fakegithub_test

import (
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/fakegithub"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

func newContentsReader(server *fakegithub.Server) *githubingest.GitHubFileReader {
	reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
		AppID:          12345,
		InstallationID: 42,
		PrivateKey:     contentsFixtureRSAKey(),
		BaseURL:        server.URL(),
	})
	Expect(err).NotTo(HaveOccurred())
	return reader
}
