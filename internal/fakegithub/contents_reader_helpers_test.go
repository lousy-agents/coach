package fakegithub_test

import (
	"github.com/lousy-agents/coach/internal/fakegithub"
	"github.com/lousy-agents/coach/pkg/githubingest"
	. "github.com/onsi/gomega"
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
