package fakegithub_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/fakegithub"
)

func newIntegrationFixture() *fakegithub.Fixture {
	fx := fakegithub.NewFixture("integration-fixture")
	fx.OAuth.ClientID = "integration-client-id"
	fx.OAuth.ClientSecret = "integration-client-secret"
	fx.OAuth.Identities["octocat"] = fakegithub.Identity{ID: 1, Login: "octocat"}
	fx.OAuth.Codes["code-ok"] = fakegithub.OAuthCodeEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioOK}
	fx.OAuth.Tokens["token-ok"] = fakegithub.OAuthTokenEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioOK}

	fx.Installation.Installations[123] = fakegithub.InstallationEntry{Token: "installation-token-abc", Scenario: fakegithub.ScenarioOK}
	fx.Installation.RepoMappings["acme/widgets"] = fakegithub.RepoInstallationEntry{InstallationID: 123, Scenario: fakegithub.ScenarioOK}
	fx.Installation.Permissions["acme/widgets/octocat"] = fakegithub.PermissionEntry{Level: "write", Scenario: fakegithub.ScenarioOK}

	fx.Contents.Files["acme/widgets/main/dir/hello.txt"] = fakegithub.FileEntry{
		Content:  []byte("integration hello"),
		SHA:      "integration-sha",
		Scenario: fakegithub.ScenarioOK,
	}

	return &fx
}

// mintInstallationToken uses the real mint endpoint so misuse specs reject a
// flow-issued token, not only a pre-registered constant.
func mintInstallationToken(server *fakegithub.Server, installationID int64) string {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v3/app/installations/%d/access_tokens", server.URL(), installationID), nil)
	Expect(err).NotTo(HaveOccurred())
	req.Header.Set("Authorization", "Bearer fake-app-jwt")

	resp, err := http.DefaultClient.Do(req)
	Expect(err).NotTo(HaveOccurred())
	defer resp.Body.Close()
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))

	var body struct {
		Token string `json:"token"`
	}
	decodeJSON(resp, &body)
	Expect(body.Token).NotTo(BeEmpty())
	return body.Token
}

// mintOAuthToken uses the real authorize→token exchange for the same reason
// as mintInstallationToken.
func mintOAuthToken(server *fakegithub.Server) string {
	authorizeURL := server.URL() + "/login/oauth/authorize?" + url.Values{
		"client_id":     {"integration-client-id"},
		"redirect_uri":  {"https://coach.example.com/callback"},
		"state":         {"xyz-state"},
		"scenario_code": {"code-ok"},
	}.Encode()

	authorizeResp, err := noRedirectClient().Get(authorizeURL)
	Expect(err).NotTo(HaveOccurred())
	authorizeResp.Body.Close()
	Expect(authorizeResp.StatusCode).To(Equal(http.StatusFound))

	exchangeResp, err := http.PostForm(server.URL()+"/login/oauth/access_token", url.Values{
		"client_id":     {"integration-client-id"},
		"client_secret": {"integration-client-secret"},
		"code":          {"code-ok"},
	})
	Expect(err).NotTo(HaveOccurred())
	defer exchangeResp.Body.Close()
	Expect(exchangeResp.StatusCode).To(Equal(http.StatusOK))

	var body struct {
		AccessToken string `json:"access_token"`
	}
	decodeJSON(exchangeResp, &body)
	Expect(body.AccessToken).NotTo(BeEmpty())
	return body.AccessToken
}

var _ = Describe("fake GitHub service integration", func() {
	var (
		fx     *fakegithub.Fixture
		server *fakegithub.Server
	)

	BeforeEach(func() {
		fx = newIntegrationFixture()
		server = fakegithub.NewServer(fx)
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("no public GitHub request", func() {
		It("serves a legitimate request over acceptanceharness.GuardedTransport, and blocks + records an attempt to a real public GitHub host on the same guarded client", func() {
			guarded := acceptanceharness.NewGuardedTransport([]string{server.Host()}, http.DefaultTransport)
			client := &http.Client{Transport: guarded}

			req, err := http.NewRequest(http.MethodGet, server.URL()+"/user", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "token token-ok")

			resp, err := client.Do(req)
			Expect(err).NotTo(HaveOccurred(), "the guarded transport must not interfere with legitimate traffic to the allowlisted fake server")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			publicReq, err := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
			Expect(err).NotTo(HaveOccurred())

			publicResp, err := client.Do(publicReq)
			Expect(err).To(HaveOccurred(), "a request to a real, non-allowlisted GitHub host must fail, never succeed with a response")
			Expect(publicResp).To(BeNil())

			Expect(guarded.BlockedRequests()).To(ContainElement("https://api.github.com/user"))
		})
	})

	Describe("cross-cutting misuse, using tokens genuinely minted by a real flow", func() {
		It("rejects a live-minted OAuth access token used against the installation-only collaborator-permission endpoint", func() {
			oauthToken := mintOAuthToken(server)

			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v3/repos/acme/widgets/collaborators/octocat/permission", server.URL()), nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "token "+oauthToken)

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(SatisfyAny(Equal(http.StatusUnauthorized), Equal(http.StatusForbidden)))

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			Expect(records[len(records)-1].AuthMode).To(Equal(acceptanceharness.AuthModeRejected))
		})

		It("rejects a live-minted GitHub App installation token used against the OAuth-only /user endpoint", func() {
			installationToken := mintInstallationToken(server, 123)

			req, err := http.NewRequest(http.MethodGet, server.URL()+"/user", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "token "+installationToken)

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(SatisfyAny(Equal(http.StatusUnauthorized), Equal(http.StatusForbidden)))

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			Expect(records[len(records)-1].AuthMode).To(Equal(acceptanceharness.AuthModeRejected))
		})

		It("rejects a live-minted OAuth access token used against the installation-only repository-contents-read endpoint", func() {
			oauthToken := mintOAuthToken(server)

			req, err := http.NewRequest(http.MethodGet, server.URL()+"/api/v3/repos/acme/widgets/contents/dir/hello.txt?ref=main", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "token "+oauthToken)

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(SatisfyAny(Equal(http.StatusUnauthorized), Equal(http.StatusForbidden)))

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			Expect(records[len(records)-1].AuthMode).To(Equal(acceptanceharness.AuthModeRejected))
		})
	})

	// RejectedTokens stand in for Coach JWTs (and any non-GitHub credential)
	// until coach-api exists; must reject on every route, not as App JWT.
	Describe("fixture-registered non-GitHub credentials (Coach JWT stand-in)", func() {
		const coachJWTStandIn = "coach-jwt-fixture-stand-in"

		BeforeEach(func() {
			fx.RejectedTokens[coachJWTStandIn] = struct{}{}
		})

		assertRejected := func(method, path, authHeader string) {
			GinkgoHelper()
			req, err := http.NewRequest(method, server.URL()+path, nil)
			Expect(err).NotTo(HaveOccurred())
			if authHeader != "" {
				req.Header.Set("Authorization", authHeader)
			}
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(SatisfyAny(Equal(http.StatusUnauthorized), Equal(http.StatusForbidden)))
			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			Expect(records[len(records)-1].AuthMode).To(Equal(acceptanceharness.AuthModeRejected))
		}

		It("rejects the stand-in against App-level installation-token mint (not as an unverifiable App JWT)", func() {
			assertRejected(http.MethodPost, "/api/v3/app/installations/123/access_tokens", "Bearer "+coachJWTStandIn)
		})

		It("rejects the stand-in against repo-to-installation resolution", func() {
			assertRejected(http.MethodGet, "/api/v3/repos/acme/widgets/installation", "Bearer "+coachJWTStandIn)
		})

		It("rejects the stand-in against collaborator-permission", func() {
			assertRejected(http.MethodGet, "/api/v3/repos/acme/widgets/collaborators/octocat/permission", "token "+coachJWTStandIn)
		})

		It("rejects the stand-in against repository contents", func() {
			assertRejected(http.MethodGet, "/api/v3/repos/acme/widgets/contents/dir/hello.txt?ref=main", "token "+coachJWTStandIn)
		})

		It("rejects the stand-in against OAuth /user", func() {
			assertRejected(http.MethodGet, "/user", "token "+coachJWTStandIn)
		})
	})

	Describe("recorder sequence across all five endpoint families", func() {
		It("records the full happy-path request sequence with correctly-classified AuthModes, in order", func() {
			authorizeURL := server.URL() + "/login/oauth/authorize?" + url.Values{
				"client_id":     {"integration-client-id"},
				"redirect_uri":  {"https://coach.example.com/callback"},
				"state":         {"xyz-state"},
				"scenario_code": {"code-ok"},
			}.Encode()
			authorizeResp, err := noRedirectClient().Get(authorizeURL)
			Expect(err).NotTo(HaveOccurred())
			authorizeResp.Body.Close()
			Expect(authorizeResp.StatusCode).To(Equal(http.StatusFound))

			exchangeResp, err := http.PostForm(server.URL()+"/login/oauth/access_token", url.Values{
				"client_id":     {"integration-client-id"},
				"client_secret": {"integration-client-secret"},
				"code":          {"code-ok"},
			})
			Expect(err).NotTo(HaveOccurred())
			var exchangeBody struct {
				AccessToken string `json:"access_token"`
			}
			decodeJSON(exchangeResp, &exchangeBody)
			Expect(exchangeBody.AccessToken).NotTo(BeEmpty())

			installationToken := mintInstallationToken(server, 123)

			resolveResp, err := http.Get(server.URL() + "/api/v3/repos/acme/widgets/installation")
			Expect(err).NotTo(HaveOccurred())
			resolveResp.Body.Close()
			Expect(resolveResp.StatusCode).To(Equal(http.StatusOK))

			userReq, err := http.NewRequest(http.MethodGet, server.URL()+"/user", nil)
			Expect(err).NotTo(HaveOccurred())
			userReq.Header.Set("Authorization", "token "+exchangeBody.AccessToken)
			userResp, err := http.DefaultClient.Do(userReq)
			Expect(err).NotTo(HaveOccurred())
			userResp.Body.Close()
			Expect(userResp.StatusCode).To(Equal(http.StatusOK))

			permReq, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v3/repos/acme/widgets/collaborators/octocat/permission", server.URL()), nil)
			Expect(err).NotTo(HaveOccurred())
			permReq.Header.Set("Authorization", "token "+installationToken)
			permResp, err := http.DefaultClient.Do(permReq)
			Expect(err).NotTo(HaveOccurred())
			permResp.Body.Close()
			Expect(permResp.StatusCode).To(Equal(http.StatusOK))

			contentsReq, err := http.NewRequest(http.MethodGet, server.URL()+"/api/v3/repos/acme/widgets/contents/dir/hello.txt?ref=main", nil)
			Expect(err).NotTo(HaveOccurred())
			contentsReq.Header.Set("Authorization", "token "+installationToken)
			contentsResp, err := http.DefaultClient.Do(contentsReq)
			Expect(err).NotTo(HaveOccurred())
			contentsResp.Body.Close()
			Expect(contentsResp.StatusCode).To(Equal(http.StatusOK))

			records := server.Recorder().Records()
			Expect(records).To(HaveLen(7))

			wantModes := []acceptanceharness.AuthMode{
				acceptanceharness.AuthModeNone,         // authorize
				acceptanceharness.AuthModeNone,         // token exchange
				acceptanceharness.AuthModeNone,         // installation-token mint
				acceptanceharness.AuthModeNone,         // repo-installation resolution
				acceptanceharness.AuthModeOAuth,        // /user
				acceptanceharness.AuthModeInstallation, // collaborator permission
				acceptanceharness.AuthModeInstallation, // contents read
			}
			gotModes := make([]acceptanceharness.AuthMode, len(records))
			for i, rec := range records {
				gotModes[i] = rec.AuthMode
			}
			Expect(gotModes).To(Equal(wantModes), "recorded sequence: %+v", records)

			for _, rec := range records {
				Expect(rec.FixtureID).To(Equal("integration-fixture"))
			}
		})
	})

	Describe("concurrent requests against one Server", func() {
		// Guards Fixture.mu on OAuth.Tokens/Codes under concurrent httptest handlers.
		It("completes many concurrent authorize->exchange->/user cycles cleanly, with every identity resolved correctly", func() {
			const concurrency = 50

			fx := newIntegrationFixture()
			for i := 0; i < concurrency; i++ {
				fx.OAuth.Codes[fmt.Sprintf("concurrent-code-%d", i)] = fakegithub.OAuthCodeEntry{
					IdentityLogin: "octocat",
					Scenario:      fakegithub.ScenarioOK,
				}
			}
			server := fakegithub.NewServer(fx)
			defer server.Close()

			type outcome struct {
				err       error
				status    int
				login     string
				id        int64
				authorize int
				exchange  int
			}
			results := make([]outcome, concurrency)

			var wg sync.WaitGroup
			for i := 0; i < concurrency; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()

					code := fmt.Sprintf("concurrent-code-%d", i)
					out := outcome{}

					authorizeURL := server.URL() + "/login/oauth/authorize?" + url.Values{
						"client_id":     {"integration-client-id"},
						"redirect_uri":  {"https://coach.example.com/callback"},
						"state":         {"xyz-state"},
						"scenario_code": {code},
					}.Encode()
					authorizeResp, err := noRedirectClient().Get(authorizeURL)
					if err != nil {
						out.err = fmt.Errorf("authorize: %w", err)
						results[i] = out
						return
					}
					authorizeResp.Body.Close()
					out.authorize = authorizeResp.StatusCode

					exchangeResp, err := http.PostForm(server.URL()+"/login/oauth/access_token", url.Values{
						"client_id":     {"integration-client-id"},
						"client_secret": {"integration-client-secret"},
						"code":          {code},
					})
					if err != nil {
						out.err = fmt.Errorf("exchange: %w", err)
						results[i] = out
						return
					}
					out.exchange = exchangeResp.StatusCode
					var exchangeBody struct {
						AccessToken string `json:"access_token"`
					}
					if err := func() error {
						defer exchangeResp.Body.Close()
						return jsonDecode(exchangeResp, &exchangeBody)
					}(); err != nil {
						out.err = fmt.Errorf("decode exchange body: %w", err)
						results[i] = out
						return
					}

					userReq, err := http.NewRequest(http.MethodGet, server.URL()+"/user", nil)
					if err != nil {
						out.err = fmt.Errorf("new /user request: %w", err)
						results[i] = out
						return
					}
					userReq.Header.Set("Authorization", "token "+exchangeBody.AccessToken)
					userResp, err := http.DefaultClient.Do(userReq)
					if err != nil {
						out.err = fmt.Errorf("/user: %w", err)
						results[i] = out
						return
					}
					out.status = userResp.StatusCode
					var user struct {
						ID    int64  `json:"id"`
						Login string `json:"login"`
					}
					if err := func() error {
						defer userResp.Body.Close()
						return jsonDecode(userResp, &user)
					}(); err != nil {
						out.err = fmt.Errorf("decode /user body: %w", err)
						results[i] = out
						return
					}
					out.login = user.Login
					out.id = user.ID

					results[i] = out
				}(i)
			}
			wg.Wait()

			for i, out := range results {
				Expect(out.err).NotTo(HaveOccurred(), "goroutine %d", i)
				Expect(out.authorize).To(Equal(http.StatusFound), "goroutine %d authorize", i)
				Expect(out.exchange).To(Equal(http.StatusOK), "goroutine %d exchange", i)
				Expect(out.status).To(Equal(http.StatusOK), "goroutine %d /user", i)
				Expect(out.login).To(Equal("octocat"), "goroutine %d /user login", i)
				Expect(out.id).To(Equal(int64(1)), "goroutine %d /user id", i)
			}

			Expect(fx.OAuth.Tokens).To(HaveLen(concurrency + 1)) // +1 for pre-registered token-ok
			for i := 0; i < concurrency; i++ {
				Expect(fx.OAuth.Codes).NotTo(HaveKey(fmt.Sprintf("concurrent-code-%d", i)))
			}
		})
	})
})

// jsonDecode decodes without closing Body (caller-owned; safe from goroutines).
func jsonDecode(resp *http.Response, out any) error {
	return json.NewDecoder(resp.Body).Decode(out)
}
