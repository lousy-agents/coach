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

// credentialsRSAKey is the credentials.go-focused variant of ginkgoRSAKey
// (acceptance_test.go): a freshly generated RSA private key, PKCS#1-PEM
// encoded like a real GitHub App private key. Never touches the network.
func credentialsRSAKey() []byte {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return pem.EncodeToMemory(block)
}

func newCredentialsFixture() *fakegithub.Fixture {
	fx := fakegithub.NewFixture("credentials-fixture")

	fx.Installation.Installations[123] = fakegithub.InstallationEntry{Token: "credentials-installation-token", Scenario: fakegithub.ScenarioOK}

	fx.Installation.RepoMappings["acme/widgets"] = fakegithub.RepoInstallationEntry{InstallationID: 123, Scenario: fakegithub.ScenarioOK}
	fx.Installation.RepoMappings["acme/transient-repo"] = fakegithub.RepoInstallationEntry{InstallationID: 123, Scenario: fakegithub.ScenarioTransient}

	return &fx
}

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

var _ = Describe("CredentialResolver (ADR-002 rule 5's single installation-token seam)", func() {
	var (
		fx     *fakegithub.Fixture
		server *fakegithub.Server
		ctx    context.Context
	)

	BeforeEach(func() {
		fx = newCredentialsFixture()
		server = fakegithub.NewServer(fx)
		ctx = context.Background()
	})

	AfterEach(func() {
		server.Close()
	})

	Context("when owner/repo is mapped to an installation the App can read", func() {
		It("resolves the installation ID and mints a fresh installation token", func() {
			resolver := newCredentialResolver(server)

			id, err := resolver.ResolveInstallationID(ctx, "acme", "widgets")
			Expect(err).NotTo(HaveOccurred())
			Expect(id).To(Equal(int64(123)))

			token, err := resolver.InstallationToken(ctx, id)
			Expect(err).NotTo(HaveOccurred())
			Expect(token).To(Equal("credentials-installation-token"))
		})
	})

	Context("when GitHub has no installation mapping for owner/repo (404)", func() {
		It("returns an error matching ErrNotFound", func() {
			resolver := newCredentialResolver(server)

			_, err := resolver.ResolveInstallationID(ctx, "acme", "unmapped-repo")

			Expect(errors.Is(err, githubingest.ErrNotFound)).To(BeTrue(), "got err %v, want errors.Is(err, ErrNotFound)", err)
		})
	})

	Context("when GitHub reports a transient failure resolving the installation", func() {
		It("returns an error matching neither ErrNotFound nor ErrAuth, for the caller to map to 503", func() {
			resolver := newCredentialResolver(server)

			_, err := resolver.ResolveInstallationID(ctx, "acme", "transient-repo")

			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, githubingest.ErrNotFound)).To(BeFalse())
			Expect(errors.Is(err, githubingest.ErrAuth)).To(BeFalse())
		})
	})
})
