package fakegithub_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/fakegithub"
)

func noRedirectClient() *http.Client {
	return &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func newOAuthFixture() *fakegithub.Fixture {
	fx := fakegithub.NewFixture("oauth-fixture")
	fx.OAuth.ClientID = "test-client-id"
	fx.OAuth.ClientSecret = "test-client-secret"
	fx.OAuth.Identities["octocat"] = fakegithub.Identity{ID: 1, Login: "octocat"}

	fx.OAuth.Codes["code-ok"] = fakegithub.OAuthCodeEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioOK}
	fx.OAuth.Codes["code-notfound"] = fakegithub.OAuthCodeEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioNotFound}
	fx.OAuth.Codes["code-authfail"] = fakegithub.OAuthCodeEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioAuthFail}
	fx.OAuth.Codes["code-transient"] = fakegithub.OAuthCodeEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioTransient}

	fx.OAuth.Tokens["token-ok"] = fakegithub.OAuthTokenEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioOK}
	fx.OAuth.Tokens["token-missing-identity"] = fakegithub.OAuthTokenEntry{IdentityLogin: "nobody-registered", Scenario: fakegithub.ScenarioOK}

	return &fx
}

func decodeJSON(body *http.Response, out any) {
	defer body.Body.Close()
	Expect(json.NewDecoder(body.Body).Decode(out)).To(Succeed())
}

var _ = Describe("fake GitHub OAuth flow", func() {
	var (
		fx     *fakegithub.Fixture
		server *fakegithub.Server
	)

	BeforeEach(func() {
		fx = newOAuthFixture()
		server = fakegithub.NewServer(fx)
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("GET /login/oauth/authorize", func() {
		It("redirects to redirect_uri with the fixture-selected code and the given state, for a matching client_id", func() {
			authorizeURL := server.URL() + "/login/oauth/authorize?" + url.Values{
				"client_id":     {"test-client-id"},
				"redirect_uri":  {"https://coach.example.com/callback"},
				"state":         {"xyz-state"},
				"scenario_code": {"code-ok"},
			}.Encode()

			resp, err := noRedirectClient().Get(authorizeURL)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusFound))

			loc, err := url.Parse(resp.Header.Get("Location"))
			Expect(err).NotTo(HaveOccurred())
			Expect(loc.Query().Get("code")).To(Equal("code-ok"))
			Expect(loc.Query().Get("state")).To(Equal("xyz-state"))
		})

		It("records the successful redirect, like every other path in this handler", func() {
			authorizeURL := server.URL() + "/login/oauth/authorize?" + url.Values{
				"client_id":     {"test-client-id"},
				"redirect_uri":  {"https://coach.example.com/callback"},
				"state":         {"xyz-state"},
				"scenario_code": {"code-ok"},
			}.Encode()

			resp, err := noRedirectClient().Get(authorizeURL)
			Expect(err).NotTo(HaveOccurred())
			resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusFound))

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			last := records[len(records)-1]
			Expect(last.AuthMode).To(Equal(acceptanceharness.AuthModeNone))
			Expect(last.FixtureID).To(Equal("oauth-fixture"))
			Expect(last.Method).To(Equal(http.MethodGet))
			Expect(last.Path).To(Equal("/login/oauth/authorize"))
			Expect(last.Scenario).To(Equal(string(fakegithub.ScenarioOK)))
		})

		It("rejects a client_id that doesn't match the fixture, without redirecting to the attacker-controlled redirect_uri", func() {
			authorizeURL := server.URL() + "/login/oauth/authorize?" + url.Values{
				"client_id":     {"wrong-client-id"},
				"redirect_uri":  {"https://attacker.example.com/callback"},
				"state":         {"xyz-state"},
				"scenario_code": {"code-ok"},
			}.Encode()

			resp, err := noRedirectClient().Get(authorizeURL)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).NotTo(Or(Equal(http.StatusFound), Equal(http.StatusMovedPermanently)))

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			last := records[len(records)-1]
			Expect(last.AuthMode).To(Equal(acceptanceharness.AuthModeRejected))
			Expect(last.FixtureID).To(Equal("oauth-fixture"))
			Expect(last.Method).To(Equal(http.MethodGet))
		})

		It("rejects a scenario_code the fixture never registered, without redirecting", func() {
			authorizeURL := server.URL() + "/login/oauth/authorize?" + url.Values{
				"client_id":     {"test-client-id"},
				"redirect_uri":  {"https://coach.example.com/callback"},
				"state":         {"xyz-state"},
				"scenario_code": {"never-registered-scenario-code"},
			}.Encode()

			resp, err := noRedirectClient().Get(authorizeURL)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).NotTo(Or(Equal(http.StatusFound), Equal(http.StatusMovedPermanently)))
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			last := records[len(records)-1]
			Expect(last.AuthMode).To(Equal(acceptanceharness.AuthModeRejected))
			Expect(last.FixtureID).To(Equal("oauth-fixture"))
			Expect(last.Method).To(Equal(http.MethodGet))
		})

		It("rejects an empty redirect_uri without issuing a Location header", func() {
			authorizeURL := server.URL() + "/login/oauth/authorize?" + url.Values{
				"client_id":     {"test-client-id"},
				"redirect_uri":  {""},
				"state":         {"xyz-state"},
				"scenario_code": {"code-ok"},
			}.Encode()

			resp, err := noRedirectClient().Get(authorizeURL)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).NotTo(Or(Equal(http.StatusFound), Equal(http.StatusMovedPermanently)))
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			Expect(resp.Header.Get("Location")).To(BeEmpty())

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			Expect(records[len(records)-1].AuthMode).To(Equal(acceptanceharness.AuthModeRejected))
		})
	})

	Describe("POST /login/oauth/access_token", func() {
		exchange := func(clientID, clientSecret, code string) *http.Response {
			resp, err := http.PostForm(server.URL()+"/login/oauth/access_token", url.Values{
				"client_id":     {clientID},
				"client_secret": {clientSecret},
				"code":          {code},
			})
			Expect(err).NotTo(HaveOccurred())
			return resp
		}

		It("exchanges a valid code (ScenarioOK) for an access token", func() {
			resp := exchange("test-client-id", "test-client-secret", "code-ok")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var body struct {
				AccessToken string `json:"access_token"`
			}
			decodeJSON(resp, &body)
			Expect(body.AccessToken).NotTo(BeEmpty())
		})

		It("lets the minted access token be used immediately against GET /user", func() {
			resp := exchange("test-client-id", "test-client-secret", "code-ok")
			var body struct {
				AccessToken string `json:"access_token"`
			}
			decodeJSON(resp, &body)
			Expect(body.AccessToken).NotTo(BeEmpty())

			req, err := http.NewRequest(http.MethodGet, server.URL()+"/user", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "token "+body.AccessToken)

			userResp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer userResp.Body.Close()
			Expect(userResp.StatusCode).To(Equal(http.StatusOK))

			var user struct {
				ID    int64  `json:"id"`
				Login string `json:"login"`
			}
			decodeJSON(userResp, &user)
			Expect(user.Login).To(Equal("octocat"))
			Expect(user.ID).To(Equal(int64(1)))
		})

		It("rejects a client_secret that doesn't match the fixture", func() {
			resp := exchange("test-client-id", "totally-wrong-secret", "code-ok")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("models ScenarioNotFound for a code registered as not-found", func() {
			resp := exchange("test-client-id", "test-client-secret", "code-notfound")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("returns 404 for a code the fixture never registered at all", func() {
			resp := exchange("test-client-id", "test-client-secret", "never-registered-code")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("models ScenarioAuthFail for a code registered as auth-failure", func() {
			resp := exchange("test-client-id", "test-client-secret", "code-authfail")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("models ScenarioTransient for a code registered as transient", func() {
			resp := exchange("test-client-id", "test-client-secret", "code-transient")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusServiceUnavailable))
		})

		It("records every exchange attempt with AuthModeNone (no bearer credential is presented)", func() {
			resp := exchange("test-client-id", "test-client-secret", "code-ok")
			resp.Body.Close()

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			last := records[len(records)-1]
			Expect(last.AuthMode).To(Equal(acceptanceharness.AuthModeNone))
			Expect(last.Method).To(Equal(http.MethodPost))
			Expect(last.Path).To(Equal("/login/oauth/access_token"))
			Expect(last.FixtureID).To(Equal("oauth-fixture"))
			Expect(last.Scenario).To(Equal(string(fakegithub.ScenarioOK)))
		})

		It("records a malformed request body that fails ParseForm, rather than leaving it unrecorded", func() {
			req, err := http.NewRequest(http.MethodPost, server.URL()+"/login/oauth/access_token", strings.NewReader("code=code-ok"))
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Content-Type", "bogus ;=")

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			last := records[len(records)-1]
			Expect(last.AuthMode).To(Equal(acceptanceharness.AuthModeNone))
			Expect(last.Method).To(Equal(http.MethodPost))
			Expect(last.Path).To(Equal("/login/oauth/access_token"))
		})
	})

	Describe("GET /user", func() {
		doUserRequest := func(authHeaderValue string) *http.Response {
			req, err := http.NewRequest(http.MethodGet, server.URL()+"/user", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", authHeaderValue)
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			return resp
		}

		It("returns the identity for a valid, directly-registered OAuth token, via the 'token' scheme", func() {
			resp := doUserRequest("token token-ok")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var user struct {
				ID    int64  `json:"id"`
				Login string `json:"login"`
			}
			decodeJSON(resp, &user)
			Expect(user.Login).To(Equal("octocat"))
			Expect(user.ID).To(Equal(int64(1)))
		})

		It("also accepts the 'Bearer' scheme for a valid OAuth token", func() {
			resp := doUserRequest("Bearer token-ok")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("rejects an unknown token", func() {
			resp := doUserRequest("token not-a-real-token")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			Expect(records[len(records)-1].AuthMode).To(Equal(acceptanceharness.AuthModeRejected))
		})

		It("returns an error status, not a 200 with an empty identity, for a token pointing at a missing identity", func() {
			resp := doUserRequest("token token-missing-identity")
			defer resp.Body.Close()
			Expect(resp.StatusCode).NotTo(Equal(http.StatusOK))
			Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
		})

		It("records a successful call with AuthModeOAuth", func() {
			resp := doUserRequest("token token-ok")
			resp.Body.Close()

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			last := records[len(records)-1]
			Expect(last.AuthMode).To(Equal(acceptanceharness.AuthModeOAuth))
			Expect(last.Method).To(Equal(http.MethodGet))
			Expect(last.Path).To(Equal("/user"))
		})

		Context("when a GitHub App installation token is misused against /user", func() {
			It("rejects it and records AuthModeRejected, rather than treating it as an OAuth identity", func() {
				misuseFx := fakegithub.NewFixture("oauth-misuse-fixture")
				misuseFx.OAuth.ClientID = "test-client-id"
				misuseFx.OAuth.ClientSecret = "test-client-secret"
				misuseFx.OAuth.Identities["octocat"] = fakegithub.Identity{ID: 1, Login: "octocat"}
				misuseFx.OAuth.Tokens["token-ok"] = fakegithub.OAuthTokenEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioOK}
				misuseFx.Installation.Installations[123] = fakegithub.InstallationEntry{
					Token:    "installation-token-abc",
					Scenario: fakegithub.ScenarioOK,
				}

				misuseServer := fakegithub.NewServer(&misuseFx)
				defer misuseServer.Close()

				req, err := http.NewRequest(http.MethodGet, misuseServer.URL()+"/user", nil)
				Expect(err).NotTo(HaveOccurred())
				req.Header.Set("Authorization", "token installation-token-abc")

				resp, err := http.DefaultClient.Do(req)
				Expect(err).NotTo(HaveOccurred())
				defer resp.Body.Close()

				Expect(resp.StatusCode).To(SatisfyAny(Equal(http.StatusUnauthorized), Equal(http.StatusForbidden)))

				records := misuseServer.Recorder().Records()
				Expect(records).NotTo(BeEmpty())
				Expect(records[len(records)-1].AuthMode).To(Equal(acceptanceharness.AuthModeRejected))
			})
		})
	})
})
