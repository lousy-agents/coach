package authn_test

import (
	. "github.com/onsi/ginkgo/v2"

	"github.com/lousy-agents/coach/internal/fakegithub"
)

func newOAuthFake() (*fakegithub.Fixture, *fakegithub.Server) {
	fx := fakegithub.NewFixture("authn-oauth")
	fx.OAuth.ClientID = oauthClientID
	fx.OAuth.ClientSecret = oauthClientSecret
	fx.OAuth.Identities["octocat"] = fakegithub.Identity{ID: 42, Login: "octocat"}
	fx.OAuth.Codes[oauthScenarioCode] = fakegithub.OAuthCodeEntry{
		IdentityLogin: "octocat",
		Scenario:      fakegithub.ScenarioOK,
	}
	srv := fakegithub.NewServer(&fx)
	DeferCleanup(srv.Close)
	return &fx, srv
}
