package main

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("loadConfigFromEnv", func() {
	When("COACH_AUTH_TEST_MINT is unset", func() {
		BeforeEach(func() {
			setValidConfigEnv()
		})

		// Story 1 requires test-mint to default off; a regression flipping
		// the `== "1"` comparison (e.g. to `!= ""`) would silently enable
		// token minting for any operator who merely sets the var to
		// anything, including "0" or "false".
		It("defaults AuthTestMintEnabled to false", func() {
			cfg, err := loadConfigFromEnv()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.AuthTestMintEnabled).To(BeFalse())
		})
	})

	When("COACH_AUTH_TEST_MINT=1 is set", func() {
		BeforeEach(func() {
			setValidConfigEnv()
			setEnv("COACH_AUTH_TEST_MINT", "1")
		})

		It("enables AuthTestMintEnabled", func() {
			cfg, err := loadConfigFromEnv()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.AuthTestMintEnabled).To(BeTrue())
		})
	})

	// A regression flipping the `== "1"` comparison to `!= ""` only changes
	// behavior for a non-empty, non-"1" value, so the unset and "=1" specs
	// above cannot catch it -- these entries are what actually distinguish
	// the two conditions.
	DescribeTable("keeps AuthTestMintEnabled false for any non-\"1\" value",
		func(value string) {
			setValidConfigEnv()
			setEnv("COACH_AUTH_TEST_MINT", value)

			cfg, err := loadConfigFromEnv()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.AuthTestMintEnabled).To(BeFalse())
		},
		Entry("COACH_AUTH_TEST_MINT=0", "0"),
		Entry("COACH_AUTH_TEST_MINT=false", "false"),
		Entry("COACH_AUTH_TEST_MINT=yes", "yes"),
	)

	DescribeTable("fails fast with the missing var named when a required var is absent",
		func(missingVar string) {
			setValidConfigEnv()
			clearEnv(missingVar)

			_, err := loadConfigFromEnv()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(missingVar))
		},
		Entry("missing COACH_JWT_SIGNING_KEY", "COACH_JWT_SIGNING_KEY"),
		Entry("missing COACH_JWT_ISSUER", "COACH_JWT_ISSUER"),
		Entry("missing COACH_HTTP_ADDR", "COACH_HTTP_ADDR"),
	)

	When("only COACH_GITHUB_OAUTH_CLIENT_ID is set", func() {
		BeforeEach(func() {
			setValidConfigEnv()
			setEnv("COACH_GITHUB_OAUTH_CLIENT_ID", "coach-oauth-client-id")
		})

		It("errors instead of silently leaving OAuth unconfigured or half-configured", func() {
			_, err := loadConfigFromEnv()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("COACH_GITHUB_OAUTH_CLIENT_ID"))
			Expect(err.Error()).To(ContainSubstring("COACH_GITHUB_OAUTH_CLIENT_SECRET"))
		})
	})

	When("only COACH_GITHUB_OAUTH_CLIENT_SECRET is set", func() {
		BeforeEach(func() {
			setValidConfigEnv()
			setEnv("COACH_GITHUB_OAUTH_CLIENT_SECRET", "coach-oauth-client-secret")
		})

		It("errors instead of silently leaving OAuth unconfigured or half-configured", func() {
			_, err := loadConfigFromEnv()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("COACH_GITHUB_OAUTH_CLIENT_ID"))
			Expect(err.Error()).To(ContainSubstring("COACH_GITHUB_OAUTH_CLIENT_SECRET"))
		})
	})
})

var _ = Describe("loadInfraConfigFromEnv", func() {
	DescribeTable("fails fast with the missing var named when a required var is absent",
		func(missingVars []string, wantSubstr string) {
			setValidInfraConfigEnv()
			clearEnv(missingVars...)

			_, err := loadInfraConfigFromEnv()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(wantSubstr))
		},
		Entry("missing COACH_GITHUB_APP_ID", []string{"COACH_GITHUB_APP_ID"}, "COACH_GITHUB_APP_ID"),
		Entry("missing COACH_REDIS_ADDR", []string{"COACH_REDIS_ADDR"}, "COACH_REDIS_ADDR"),
		Entry("missing private key (neither raw value nor path set)",
			[]string{"COACH_GITHUB_APP_PRIVATE_KEY", "COACH_GITHUB_APP_PRIVATE_KEY_PATH"},
			"COACH_GITHUB_APP_PRIVATE_KEY"),
	)

	// Story 4 / Task 10 credential-free smoke: when the full authz bypass pair
	// is configured, GitHub App credentials must be optional so compose core
	// can start with zero GitHub secrets.
	When("both COACH_AUTHZ_BYPASS_OWNER and COACH_AUTHZ_BYPASS_REPO are set", func() {
		BeforeEach(func() {
			setValidInfraConfigEnv()
			setEnv("COACH_AUTHZ_BYPASS_OWNER", "coach-smoke")
			setEnv("COACH_AUTHZ_BYPASS_REPO", "fixture-repo")
			clearEnv("COACH_GITHUB_APP_ID", "COACH_GITHUB_APP_PRIVATE_KEY", "COACH_GITHUB_APP_PRIVATE_KEY_PATH")
		})

		It("loads InfraConfig with zero GitHub App credentials", func() {
			cfg, err := loadInfraConfigFromEnv()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg.GitHubAppID).To(BeZero())
			Expect(cfg.GitHubAppPrivateKey).To(BeEmpty())
			Expect(cfg.AuthzBypassOwner).To(Equal("coach-smoke"))
			Expect(cfg.AuthzBypassRepo).To(Equal("fixture-repo"))
			Expect(cfg.RedisAddr).To(Equal(configTestRedisAddr))
		})
	})

	When("only COACH_AUTHZ_BYPASS_OWNER is set and App credentials are absent", func() {
		BeforeEach(func() {
			setValidInfraConfigEnv()
			setEnv("COACH_AUTHZ_BYPASS_OWNER", "coach-smoke")
			clearEnv("COACH_AUTHZ_BYPASS_REPO", "COACH_GITHUB_APP_ID",
				"COACH_GITHUB_APP_PRIVATE_KEY", "COACH_GITHUB_APP_PRIVATE_KEY_PATH")
		})

		It("fails fast rather than starting half-configured without App credentials", func() {
			_, err := loadInfraConfigFromEnv()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Or(
				ContainSubstring("COACH_GITHUB_APP_ID"),
				ContainSubstring("COACH_AUTHZ_BYPASS"),
			))
		})
	})

	When("neither App credentials nor the full bypass pair are set", func() {
		BeforeEach(func() {
			setValidInfraConfigEnv()
			clearEnv("COACH_GITHUB_APP_ID", "COACH_GITHUB_APP_PRIVATE_KEY",
				"COACH_GITHUB_APP_PRIVATE_KEY_PATH", "COACH_AUTHZ_BYPASS_OWNER", "COACH_AUTHZ_BYPASS_REPO")
		})

		It("fails fast with a clear error naming App credentials or the bypass pair", func() {
			_, err := loadInfraConfigFromEnv()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Or(
				ContainSubstring("COACH_GITHUB_APP_ID"),
				ContainSubstring("COACH_AUTHZ_BYPASS"),
			))
		})
	})
})

var _ = Describe("buildAuthorizer", func() {
	// Credential-free smoke wiring must never construct a live GitHub
	// CredentialResolver: only the configured bypass pair is authorized, and
	// every other owner/repo fails closed.
	When("InfraConfig has the full bypass pair and no GitHub App credentials", func() {
		It("authorizes only the bypass pair and denies every other owner/repo", func() {
			cfg := InfraConfig{
				AuthzBypassOwner: "coach-smoke",
				AuthzBypassRepo:  "fixture-repo",
			}
			authorizer, err := buildAuthorizer(cfg)
			Expect(err).NotTo(HaveOccurred())
			Expect(authorizer).NotTo(BeNil())

			Expect(authorizer.Authorize(context.Background(), "smoke-user", "coach-smoke", "fixture-repo")).
				To(Succeed())
			Expect(authorizer.Authorize(context.Background(), "smoke-user", "acme", "widgets")).
				To(HaveOccurred())
			Expect(authorizer.Authorize(context.Background(), "smoke-user", "coach-smoke", "other-repo")).
				To(HaveOccurred())
		})
	})

	When("InfraConfig has neither App credentials nor a full bypass pair", func() {
		It("fails closed instead of constructing an open authorizer", func() {
			_, err := buildAuthorizer(InfraConfig{})
			Expect(err).To(HaveOccurred())
		})
	})
})

// recordingAuthorizer denies exactly one (owner, repo) pair, so a test can
// tell whether the authorizer it receives back is the bare inner (denies
// that pair) or something that bypassed it (allows that pair regardless).
type recordingAuthorizer struct {
	deniedOwner string
	deniedRepo  string
}

func (r recordingAuthorizer) Authorize(_ context.Context, _, owner, repo string) error {
	if owner == r.deniedOwner && repo == r.deniedRepo {
		return errors.New("denied by inner authorizer")
	}
	return nil
}

// denyAllAuthorizer denies every request, so a test can tell whether the
// authorizer it receives back is the bare inner (always denies) or
// something that bypassed it for some request (allows it).
type denyAllAuthorizer struct{}

func (denyAllAuthorizer) Authorize(context.Context, string, string, string) error {
	return errors.New("denied by inner authorizer")
}

var _ = Describe("wrapAuthorizerForBypass", func() {
	inner := recordingAuthorizer{deniedOwner: "acme", deniedRepo: "widgets"}

	// Story 3's bypass must require both owner and repo; a regression
	// flipping the guard's `&&` to `||` would construct a BypassAuthorizer
	// whose unset field defaults to "", which then matches any request
	// whose corresponding field is also empty -- so the request here
	// deliberately supplies "" for the field that was left unconfigured,
	// which is exactly the request such a broken `||` would wrongly allow.
	When("only AuthzBypassOwner is set", func() {
		It("does not bypass authorization for a request with an empty repo", func() {
			cfg := InfraConfig{AuthzBypassOwner: "acme"}
			authorizer := wrapAuthorizerForBypass(denyAllAuthorizer{}, cfg)
			err := authorizer.Authorize(context.Background(), "someone", "acme", "")
			Expect(err).To(HaveOccurred(), "owner-only bypass config must not disable authorization")
		})
	})

	When("only AuthzBypassRepo is set", func() {
		It("does not bypass authorization for a request with an empty owner", func() {
			cfg := InfraConfig{AuthzBypassRepo: "widgets"}
			authorizer := wrapAuthorizerForBypass(denyAllAuthorizer{}, cfg)
			err := authorizer.Authorize(context.Background(), "someone", "", "widgets")
			Expect(err).To(HaveOccurred(), "repo-only bypass config must not disable authorization")
		})
	})

	When("neither AuthzBypassOwner nor AuthzBypassRepo is set", func() {
		It("does not bypass the inner authorizer", func() {
			authorizer := wrapAuthorizerForBypass(inner, InfraConfig{})
			err := authorizer.Authorize(context.Background(), "someone", "acme", "widgets")
			Expect(err).To(HaveOccurred())
		})
	})

	When("both AuthzBypassOwner and AuthzBypassRepo are set to the matching pair", func() {
		It("bypasses the inner authorizer for that exact pair", func() {
			cfg := InfraConfig{AuthzBypassOwner: "acme", AuthzBypassRepo: "widgets"}
			authorizer := wrapAuthorizerForBypass(inner, cfg)
			err := authorizer.Authorize(context.Background(), "someone", "acme", "widgets")
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
