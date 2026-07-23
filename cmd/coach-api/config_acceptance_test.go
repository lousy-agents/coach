package main

import (
	"context"
	"errors"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	configTestSigningKey = "test-signing-secret-at-least-32-bytes!!"
	configTestIssuer     = "https://coach-api.test"
	configTestHTTPAddr   = ":8080"

	configTestGitHubAppID   = "123"
	configTestGitHubPrivKey = "test-private-key-pem-contents"
	configTestRedisAddr     = "127.0.0.1:6379"
)

// setEnv sets key=value for the duration of the current spec via
// GinkgoT().Setenv, which restores the prior value (or unsets it) in
// cleanup, so specs never leak env vars into one another.
func setEnv(key, value string) {
	GinkgoHelper()
	GinkgoT().Setenv(key, value)
}

// clearEnv unsets each key for the duration of the current spec,
// restoring whatever value (or absence) preceded it once the spec ends.
func clearEnv(keys ...string) {
	GinkgoHelper()
	for _, key := range keys {
		key := key
		if v, ok := os.LookupEnv(key); ok {
			DeferCleanup(func() { Expect(os.Setenv(key, v)).To(Succeed()) })
		}
		Expect(os.Unsetenv(key)).To(Succeed())
	}
}

// setValidConfigEnv sets every var loadConfigFromEnv requires, plus clears
// the optional ones this suite cares about isolating (COACH_AUTH_TEST_MINT,
// GitHub OAuth), so each spec can override exactly the one var under test.
func setValidConfigEnv() {
	GinkgoHelper()
	setEnv("COACH_JWT_SIGNING_KEY", configTestSigningKey)
	setEnv("COACH_JWT_ISSUER", configTestIssuer)
	setEnv("COACH_HTTP_ADDR", configTestHTTPAddr)
	clearEnv("COACH_AUTH_TEST_MINT", "COACH_JWT_TOKEN_TTL",
		"COACH_GITHUB_OAUTH_CLIENT_ID", "COACH_GITHUB_OAUTH_CLIENT_SECRET",
		"COACH_GITHUB_OAUTH_REDIRECT_URI", "COACH_GITHUB_OAUTH_BASE_URL",
		"COACH_GITHUB_OAUTH_API_BASE_URL")
}

// setValidInfraConfigEnv sets every var loadInfraConfigFromEnv requires.
func setValidInfraConfigEnv() {
	GinkgoHelper()
	setEnv("COACH_GITHUB_APP_ID", configTestGitHubAppID)
	setEnv("COACH_GITHUB_APP_PRIVATE_KEY", configTestGitHubPrivKey)
	setEnv("COACH_REDIS_ADDR", configTestRedisAddr)
	clearEnv("COACH_GITHUB_APP_PRIVATE_KEY_PATH", "COACH_AUTHZ_BYPASS_OWNER", "COACH_AUTHZ_BYPASS_REPO")
}

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
