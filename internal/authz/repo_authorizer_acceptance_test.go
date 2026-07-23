package authz_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/authz"
	"github.com/lousy-agents/coach/internal/fakegithub"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

// authzRSAKey returns a freshly generated RSA private key, PKCS#1-PEM
// encoded like a real GitHub App private key. Never touches the network.
func authzRSAKey() []byte {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return pem.EncodeToMemory(block)
}

// newAuthzFixture builds one fixture covering every row of ADR-003's
// validation matrix: user-owned repo, direct collaborator, org/team-derived
// access (the fake's permission endpoint does not distinguish direct from
// org/team-derived access -- both are just a registered permission level --
// so those two rows share fixture mechanics), no-role principal, an
// unmapped repo standing in for both "app not installed" and "nonexistent
// repo" (GitHub itself makes them indistinguishable), and transient failures
// at both the installation-resolution and permission-check steps.
func newAuthzFixture() *fakegithub.Fixture {
	fx := fakegithub.NewFixture("authz-fixture")

	fx.Installation.Installations[1] = fakegithub.InstallationEntry{Token: "authz-installation-token", Scenario: fakegithub.ScenarioOK}

	fx.Installation.RepoMappings["octocat/octocat-repo"] = fakegithub.RepoInstallationEntry{InstallationID: 1, Scenario: fakegithub.ScenarioOK}
	fx.Installation.RepoMappings["acme/widgets"] = fakegithub.RepoInstallationEntry{InstallationID: 1, Scenario: fakegithub.ScenarioOK}
	fx.Installation.RepoMappings["acme/transient-repo"] = fakegithub.RepoInstallationEntry{InstallationID: 1, Scenario: fakegithub.ScenarioTransient}

	fx.Installation.Permissions["octocat/octocat-repo/octocat"] = fakegithub.PermissionEntry{Level: "admin", Scenario: fakegithub.ScenarioOK}
	fx.Installation.Permissions["acme/widgets/collab-user"] = fakegithub.PermissionEntry{Level: "write", Scenario: fakegithub.ScenarioOK}
	fx.Installation.Permissions["acme/widgets/team-user"] = fakegithub.PermissionEntry{Level: "read", Scenario: fakegithub.ScenarioOK}
	fx.Installation.Permissions["acme/widgets/outsider"] = fakegithub.PermissionEntry{Level: "none", Scenario: fakegithub.ScenarioOK}
	fx.Installation.Permissions["acme/widgets/empty-perm-user"] = fakegithub.PermissionEntry{Level: "", Scenario: fakegithub.ScenarioOK}
	fx.Installation.Permissions["acme/widgets/unknown-perm-user"] = fakegithub.PermissionEntry{Level: "superadmin", Scenario: fakegithub.ScenarioOK}
	fx.Installation.Permissions["acme/widgets/transient-user"] = fakegithub.PermissionEntry{Scenario: fakegithub.ScenarioTransient}

	return &fx
}

func newAuthzAuthorizer(server *fakegithub.Server) authz.RepoAuthorizer {
	GinkgoHelper()
	credentials, err := githubingest.NewCredentialResolver(githubingest.CredentialResolverConfig{
		AppID:      12345,
		PrivateKey: authzRSAKey(),
		BaseURL:    server.URL(),
	})
	Expect(err).NotTo(HaveOccurred())

	authorizer, err := authz.NewGitHubRepoAuthorizer(authz.GitHubRepoAuthorizerConfig{
		Credentials: credentials,
		BaseURL:     server.URL(),
	})
	Expect(err).NotTo(HaveOccurred())
	return authorizer
}

var _ = Describe("GitHubRepoAuthorizer (ADR-003 submit-time repository authorization)", func() {
	var (
		fx         *fakegithub.Fixture
		server     *fakegithub.Server
		ctx        context.Context
		authorizer authz.RepoAuthorizer
	)

	BeforeEach(func() {
		fx = newAuthzFixture()
		server = fakegithub.NewServer(fx)
		ctx = context.Background()
		authorizer = newAuthzAuthorizer(server)
	})

	AfterEach(func() {
		server.Close()
	})

	When("the principal owns the repository", func() {
		It("authorizes the scan", func() {
			err := authorizer.Authorize(ctx, "octocat", "octocat", "octocat-repo")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("the principal is a direct collaborator with a role", func() {
		It("authorizes the scan", func() {
			err := authorizer.Authorize(ctx, "collab-user", "acme", "widgets")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("the principal has org/team-derived access", func() {
		It("authorizes the scan", func() {
			err := authorizer.Authorize(ctx, "team-user", "acme", "widgets")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("the principal has no role in the repository", func() {
		It("denies with an error matching ErrNotAuthorized", func() {
			err := authorizer.Authorize(ctx, "outsider", "acme", "widgets")
			Expect(errors.Is(err, authz.ErrNotAuthorized)).To(BeTrue(), "got err %v, want errors.Is(err, ErrNotAuthorized)", err)
		})
	})

	When("GitHub returns an empty effective permission level", func() {
		It("denies with an error matching ErrNotAuthorized (fail closed; empty is not a recognized role)", func() {
			err := authorizer.Authorize(ctx, "empty-perm-user", "acme", "widgets")
			Expect(errors.Is(err, authz.ErrNotAuthorized)).To(BeTrue(), "got err %v, want errors.Is(err, ErrNotAuthorized)", err)
		})
	})

	When("GitHub returns an unrecognized effective permission level", func() {
		It("denies with an error matching ErrNotAuthorized (fail closed; only known GitHub roles authorize)", func() {
			err := authorizer.Authorize(ctx, "unknown-perm-user", "acme", "widgets")
			Expect(errors.Is(err, authz.ErrNotAuthorized)).To(BeTrue(), "got err %v, want errors.Is(err, ErrNotAuthorized)", err)
		})
	})

	When("the GitHub App installation is not installed on the repository", func() {
		It("denies with an error matching ErrNotAuthorized", func() {
			err := authorizer.Authorize(ctx, "octocat", "acme", "uninstalled-repo")
			Expect(errors.Is(err, authz.ErrNotAuthorized)).To(BeTrue(), "got err %v, want errors.Is(err, ErrNotAuthorized)", err)
		})
	})

	When("the repository does not exist", func() {
		It("denies with the same ErrNotAuthorized outcome as an uninstalled repo -- GitHub gives no distinguishable signal", func() {
			err := authorizer.Authorize(ctx, "octocat", "ghost-org", "ghost-repo")
			Expect(errors.Is(err, authz.ErrNotAuthorized)).To(BeTrue(), "got err %v, want errors.Is(err, ErrNotAuthorized)", err)
		})
	})

	When("the principal has no registered relationship with an installed repository (permission 404)", func() {
		It("denies with an error matching ErrNotAuthorized", func() {
			err := authorizer.Authorize(ctx, "ghost-user", "acme", "widgets")
			Expect(errors.Is(err, authz.ErrNotAuthorized)).To(BeTrue(), "got err %v, want errors.Is(err, ErrNotAuthorized)", err)
		})
	})

	When("GitHub fails transiently while resolving the installation", func() {
		It("returns an error that does not match ErrNotAuthorized, for the caller to map to 503", func() {
			err := authorizer.Authorize(ctx, "octocat", "acme", "transient-repo")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, authz.ErrNotAuthorized)).To(BeFalse(), "transient failures must not collapse into repo_not_authorized")
		})
	})

	When("GitHub fails transiently while checking the principal's permission", func() {
		It("returns an error that does not match ErrNotAuthorized, for the caller to map to 503", func() {
			err := authorizer.Authorize(ctx, "transient-user", "acme", "widgets")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, authz.ErrNotAuthorized)).To(BeFalse(), "transient failures must not collapse into repo_not_authorized")
		})
	})
})

// stubAuthorizer is a RepoAuthorizer test double that always returns a fixed
// decision, so BypassAuthorizer specs can prove exactly which pair skips the
// live check without depending on GitHubRepoAuthorizer or the fake server.
type stubAuthorizer struct{ err error }

func (s stubAuthorizer) Authorize(context.Context, string, string, string) error {
	return s.err
}

var _ = Describe("BypassAuthorizer (Story 3's credential-free smoke / test-mint exception)", func() {
	var inner stubAuthorizer

	BeforeEach(func() {
		// inner always denies, so a bypass success can only come from the
		// wrapper itself, never from delegation.
		inner = stubAuthorizer{err: authz.ErrNotAuthorized}
	})

	When("the requested owner/repo exactly matches the configured bypass pair", func() {
		It("authorizes without consulting inner", func() {
			bypass := authz.NewBypassAuthorizer(inner, "acme", "widgets")

			err := bypass.Authorize(context.Background(), "anyone", "acme", "widgets")

			Expect(err).NotTo(HaveOccurred())
		})
	})

	When("the requested owner/repo does not match the configured bypass pair", func() {
		It("delegates to inner and surfaces inner's live decision", func() {
			bypass := authz.NewBypassAuthorizer(inner, "acme", "widgets")

			err := bypass.Authorize(context.Background(), "anyone", "acme", "other-repo")

			Expect(errors.Is(err, authz.ErrNotAuthorized)).To(BeTrue(), "non-matching pairs must run the full live-check matrix via inner")
		})
	})

	When("the match is case-sensitive", func() {
		It("does not bypass a differently-cased owner/repo", func() {
			bypass := authz.NewBypassAuthorizer(inner, "acme", "widgets")

			err := bypass.Authorize(context.Background(), "anyone", "ACME", "Widgets")

			Expect(errors.Is(err, authz.ErrNotAuthorized)).To(BeTrue(), "bypass must be exact and case-sensitive")
		})
	})
})
