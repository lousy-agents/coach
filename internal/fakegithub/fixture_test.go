package fakegithub_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/fakegithub"
)

// Fixture.ClassifyToken is exercised directly here (rather than through
// Server's HTTP surface) because the bug this spec guards against is in the
// classification function itself, independent of any handler: an empty
// token must never match a fixture-registered InstallationEntry that
// deliberately has an empty Token field (auth-fail/transient entries never
// mint a real token -- see installation_acceptance_test.go's
// newInstallationFixture).
var _ = Describe("fakegithub.Fixture.ClassifyToken", func() {
	It("classifies an empty token as TokenUnknown, even when an installation entry has an empty Token", func() {
		fx := fakegithub.NewFixture("classify-token-fixture")
		fx.Installation.Installations[401] = fakegithub.InstallationEntry{Scenario: fakegithub.ScenarioAuthFail}

		Expect(fx.ClassifyToken("")).To(Equal(fakegithub.TokenUnknown))
	})

	It("classifies a RejectedTokens entry as TokenRejected ahead of OAuth/installation registries", func() {
		fx := fakegithub.NewFixture("classify-token-fixture")
		fx.RejectedTokens["coach-jwt-stand-in"] = struct{}{}
		fx.OAuth.Tokens["coach-jwt-stand-in"] = fakegithub.OAuthTokenEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioOK}

		Expect(fx.ClassifyToken("coach-jwt-stand-in")).To(Equal(fakegithub.TokenRejected))
	})
})
