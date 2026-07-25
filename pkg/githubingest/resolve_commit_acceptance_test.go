package githubingest_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/fakegithub"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

func resolveCommitRSAKey() []byte {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return pem.EncodeToMemory(block)
}

func newResolveCommitFixture() *fakegithub.Fixture {
	fx := fakegithub.NewFixture("resolve-commit-fixture")
	fx.Installation.Installations[77] = fakegithub.InstallationEntry{
		Token:    "resolve-commit-token",
		Scenario: fakegithub.ScenarioOK,
	}
	fx.Installation.RepoMappings["acme/widgets"] = fakegithub.RepoInstallationEntry{
		InstallationID: 77,
		Scenario:       fakegithub.ScenarioOK,
	}
	const objectSHA = "0123456789abcdef0123456789abcdef01234567"
	fx.Repos.Repos["acme/widgets"] = fakegithub.RepoMetaEntry{
		DefaultBranch: "main",
		Scenario:      fakegithub.ScenarioOK,
	}
	fx.Repos.Commits["acme/widgets/main"] = fakegithub.CommitEntry{
		SHA:      objectSHA,
		Scenario: fakegithub.ScenarioOK,
	}
	fx.Repos.Commits["acme/widgets/"+objectSHA] = fakegithub.CommitEntry{
		SHA:      objectSHA,
		Scenario: fakegithub.ScenarioOK,
	}
	return &fx
}

var _ = Describe("ResolveCommitSHA + token-backed reader (ADR-002)", func() {
	var (
		fx     *fakegithub.Fixture
		server *fakegithub.Server
		ctx    context.Context
	)

	BeforeEach(func() {
		fx = newResolveCommitFixture()
		server = fakegithub.NewServer(fx)
		ctx = context.Background()
	})

	AfterEach(func() {
		server.Close()
	})

	When("a reader is built from a CredentialResolver-minted installation token", func() {
		It("resolves a branch ref to the commit object SHA", func() {
			resolver, err := githubingest.NewCredentialResolver(githubingest.CredentialResolverConfig{
				AppID:      12345,
				PrivateKey: resolveCommitRSAKey(),
				BaseURL:    server.URL(),
			})
			Expect(err).NotTo(HaveOccurred())

			id, err := resolver.ResolveInstallationID(ctx, "acme", "widgets")
			Expect(err).NotTo(HaveOccurred())
			token, err := resolver.InstallationToken(ctx, id)
			Expect(err).NotTo(HaveOccurred())

			reader, err := githubingest.NewGitHubFileReaderFromToken(token, server.URL())
			Expect(err).NotTo(HaveOccurred())

			sha, err := reader.ResolveCommitSHA(ctx, "acme", "widgets", "main")
			Expect(err).NotTo(HaveOccurred())
			Expect(sha).To(Equal("0123456789abcdef0123456789abcdef01234567"))
			Expect(sha).NotTo(Equal("main"))
			Expect(sha).NotTo(Equal("HEAD"))
		})

		It("resolves an empty ref to the default branch tip SHA (not literal HEAD)", func() {
			reader, err := githubingest.NewGitHubFileReaderFromToken("resolve-commit-token", server.URL())
			Expect(err).NotTo(HaveOccurred())

			sha, err := reader.ResolveCommitSHA(ctx, "acme", "widgets", "")
			Expect(err).NotTo(HaveOccurred())
			Expect(sha).To(Equal("0123456789abcdef0123456789abcdef01234567"))
			Expect(sha).NotTo(Equal("HEAD"))
		})
	})

	When("the ref cannot be resolved", func() {
		It("returns ErrNotFound for an unknown ref", func() {
			reader, err := githubingest.NewGitHubFileReaderFromToken("resolve-commit-token", server.URL())
			Expect(err).NotTo(HaveOccurred())

			_, err = reader.ResolveCommitSHA(ctx, "acme", "widgets", "no-such-branch")
			Expect(errors.Is(err, githubingest.ErrNotFound)).To(BeTrue(), "got %v", err)
		})
	})

	When("NewGitHubFileReaderFromToken is given an empty token", func() {
		It("fails closed without building a client", func() {
			_, err := githubingest.NewGitHubFileReaderFromToken("", server.URL())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("token"))
		})
	})
})
