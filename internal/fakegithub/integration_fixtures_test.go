package fakegithub_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/lousy-agents/coach/internal/fakegithub"
	. "github.com/onsi/gomega"
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

// jsonDecode decodes without closing Body (caller-owned; safe from goroutines).
func jsonDecode(resp *http.Response, out any) error {
	return json.NewDecoder(resp.Body).Decode(out)
}
