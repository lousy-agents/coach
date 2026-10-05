package githubingest_test

import (
	"github.com/lousy-agents/coach/internal/fakegithub"
	"github.com/lousy-agents/coach/pkg/githubingest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func newCredentialResolver(server *fakegithub.Server) *githubingest.CredentialResolver {
	GinkgoHelper()
	resolver, err := githubingest.NewCredentialResolver(githubingest.CredentialResolverConfig{
		AppID:      12345,
		PrivateKey: credentialsRSAKey(),
		BaseURL:    server.URL(),
	})
	Expect(err).NotTo(HaveOccurred())
	return resolver
}
