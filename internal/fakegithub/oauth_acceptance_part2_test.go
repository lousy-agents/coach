package fakegithub_test

import (
	"github.com/lousy-agents/coach/internal/fakegithub"
)

func newOAuthFixture() *fakegithub.Fixture {
	fx := fakegithub.NewFixture("oauth-fixture")
	fx.OAuth.ClientID = "test-client-id"
	fx.OAuth.ClientSecret = "test-client-secret"
	fx.OAuth.Identities["octocat"] = fakegithub.Identity{ID: 1, Login: "octocat"}

	fx.OAuth.Codes["code-ok"] = fakegithub.OAuthCodeEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioOK}
	fx.OAuth.Codes["code-notfound"] = fakegithub.OAuthCodeEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioNotFound}
	fx.OAuth.Codes["code-authfail"] = fakegithub.OAuthCodeEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioAuthFail}
	fx.OAuth.Codes["code-transient"] = fakegithub.OAuthCodeEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioTransient}
	fx.OAuth.Codes["code-empty-scenario"] = fakegithub.OAuthCodeEntry{IdentityLogin: "octocat"}
	fx.OAuth.Codes["code-typo-scenario"] = fakegithub.OAuthCodeEntry{IdentityLogin: "octocat", Scenario: "ok "}

	fx.OAuth.Tokens["token-ok"] = fakegithub.OAuthTokenEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioOK}
	fx.OAuth.Tokens["token-missing-identity"] = fakegithub.OAuthTokenEntry{IdentityLogin: "nobody-registered", Scenario: fakegithub.ScenarioOK}
	fx.OAuth.Tokens["token-typo-scenario"] = fakegithub.OAuthTokenEntry{IdentityLogin: "octocat", Scenario: "ok "}

	return &fx
}
