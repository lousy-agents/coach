package fakegithub_test

import (
	"fmt"
	"net/http"
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/fakegithub"
)

// newIntegrationFixture builds a single Fixture spanning all five endpoint
// families fakegithub implements (OAuth authorize/exchange/"/user", GitHub
// App installation-token mint, repo-to-installation resolution, effective
// permissions, and repository content reads), so this file's specs -- which
// are cross-cutting by design, unlike the per-family fixtures in
// oauth_acceptance_test.go/installation_acceptance_test.go/
// contents_acceptance_test.go -- can drive a request against any of them
// from one Fixture/Server pair.
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

// mintInstallationToken drives the real POST
// /api/v3/app/installations/{id}/access_tokens flow and returns the
// genuinely minted token, rather than reaching for the fixture's Token field
// directly -- this file's misuse specs exist specifically to prove a token
// obtained through a real flow is rejected elsewhere, not merely that a
// pre-registered constant is.
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

// mintOAuthToken drives the real GET /login/oauth/authorize -> POST
// /login/oauth/access_token flow and returns the genuinely minted access
// token, for the same "prove a real-flow token is rejected elsewhere"
// reason as mintInstallationToken above.
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
	})

	Describe("recorder sequence across all five endpoint families", func() {
		It("records the full happy-path request sequence with correctly-classified AuthModes, in order", func() {
			// 1: authorize (AuthModeNone).
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

			// 2: token exchange (AuthModeNone).
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

			// 3: installation-token mint (AuthModeNone).
			installationToken := mintInstallationToken(server, 123)

			// 4: repo-to-installation resolution (AuthModeNone).
			resolveResp, err := http.Get(server.URL() + "/api/v3/repos/acme/widgets/installation")
			Expect(err).NotTo(HaveOccurred())
			resolveResp.Body.Close()
			Expect(resolveResp.StatusCode).To(Equal(http.StatusOK))

			// 5: /user with the OAuth access token (AuthModeOAuth).
			userReq, err := http.NewRequest(http.MethodGet, server.URL()+"/user", nil)
			Expect(err).NotTo(HaveOccurred())
			userReq.Header.Set("Authorization", "token "+exchangeBody.AccessToken)
			userResp, err := http.DefaultClient.Do(userReq)
			Expect(err).NotTo(HaveOccurred())
			userResp.Body.Close()
			Expect(userResp.StatusCode).To(Equal(http.StatusOK))

			// 6: collaborator-permission check with the installation token
			// (AuthModeInstallation).
			permReq, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v3/repos/acme/widgets/collaborators/octocat/permission", server.URL()), nil)
			Expect(err).NotTo(HaveOccurred())
			permReq.Header.Set("Authorization", "token "+installationToken)
			permResp, err := http.DefaultClient.Do(permReq)
			Expect(err).NotTo(HaveOccurred())
			permResp.Body.Close()
			Expect(permResp.StatusCode).To(Equal(http.StatusOK))

			// 7: repository content read with the installation token
			// (AuthModeInstallation).
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
})
