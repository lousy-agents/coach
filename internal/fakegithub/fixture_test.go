package fakegithub_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/fakegithub"
)

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
