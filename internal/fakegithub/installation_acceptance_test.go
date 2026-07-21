package fakegithub_test

import (
	"encoding/json"
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/fakegithub"
)

func newInstallationFixture() *fakegithub.Fixture {
	fx := fakegithub.NewFixture("installation-fixture")

	fx.Installation.Installations[123] = fakegithub.InstallationEntry{Token: "installation-token-abc", Scenario: fakegithub.ScenarioOK}
	fx.Installation.Installations[401] = fakegithub.InstallationEntry{Scenario: fakegithub.ScenarioAuthFail}
	fx.Installation.Installations[503] = fakegithub.InstallationEntry{Scenario: fakegithub.ScenarioTransient}
	fx.Installation.Installations[404] = fakegithub.InstallationEntry{Scenario: fakegithub.ScenarioNotFound}

	fx.Installation.RepoMappings["acme/widgets"] = fakegithub.RepoInstallationEntry{InstallationID: 123, Scenario: fakegithub.ScenarioOK}
	fx.Installation.RepoMappings["acme/authfail-repo"] = fakegithub.RepoInstallationEntry{InstallationID: 123, Scenario: fakegithub.ScenarioAuthFail}
	fx.Installation.RepoMappings["acme/transient-repo"] = fakegithub.RepoInstallationEntry{InstallationID: 123, Scenario: fakegithub.ScenarioTransient}
	fx.Installation.RepoMappings["acme/notfound-repo"] = fakegithub.RepoInstallationEntry{InstallationID: 123, Scenario: fakegithub.ScenarioNotFound}

	fx.Installation.Permissions["acme/widgets/octocat"] = fakegithub.PermissionEntry{Level: "write", Scenario: fakegithub.ScenarioOK}
	fx.Installation.Permissions["acme/widgets/authfail-user"] = fakegithub.PermissionEntry{Scenario: fakegithub.ScenarioAuthFail}
	fx.Installation.Permissions["acme/widgets/transient-user"] = fakegithub.PermissionEntry{Scenario: fakegithub.ScenarioTransient}
	fx.Installation.Permissions["acme/widgets/notfound-user"] = fakegithub.PermissionEntry{Scenario: fakegithub.ScenarioNotFound}

	return &fx
}

var _ = Describe("fake GitHub App installation and authorization", func() {
	var (
		fx     *fakegithub.Fixture
		server *fakegithub.Server
	)

	BeforeEach(func() {
		fx = newInstallationFixture()
		server = fakegithub.NewServer(fx)
	})

	AfterEach(func() {
		server.Close()
	})

	mintToken := func(installationID int64) *http.Response {
		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v3/app/installations/%d/access_tokens", server.URL(), installationID), nil)
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("Authorization", "Bearer fake-app-jwt")
		resp, err := http.DefaultClient.Do(req)
		Expect(err).NotTo(HaveOccurred())
		return resp
	}

	Describe("POST /api/v3/app/installations/{id}/access_tokens", func() {
		It("mints the fixture's registered token for a known installation (ScenarioOK), without verifying the JWT", func() {
			resp := mintToken(123)
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusCreated))

			var body struct {
				Token     string `json:"token"`
				ExpiresAt string `json:"expires_at"`
			}
			Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
			Expect(body.Token).To(Equal("installation-token-abc"))
			Expect(body.ExpiresAt).NotTo(BeEmpty())
		})

		It("returns 404 for an installation ID the fixture never registered", func() {
			resp := mintToken(999999)
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("records a request with a non-numeric installation id rather than leaving the recorder empty", func() {
			req, err := http.NewRequest(http.MethodPost, server.URL()+"/api/v3/app/installations/not-a-number/access_tokens", nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", "Bearer fake-app-jwt")
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty(), "every request the fake handles must be recorded, including parse failures")
			last := records[len(records)-1]
			Expect(last.Method).To(Equal(http.MethodPost))
			Expect(last.Path).To(ContainSubstring("/app/installations/"))
			Expect(last.FixtureID).To(Equal("installation-fixture"))
		})

		It("models ScenarioAuthFail for an installation registered as auth-failure", func() {
			resp := mintToken(401)
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("models ScenarioTransient for an installation registered as transient", func() {
			resp := mintToken(503)
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusServiceUnavailable))
		})

		It("models ScenarioNotFound for an installation explicitly registered as not-found", func() {
			resp := mintToken(404)
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("records the mint with AuthModeNone (App-level JWT auth isn't in the oauth/installation vocab)", func() {
			resp := mintToken(123)
			resp.Body.Close()

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			last := records[len(records)-1]
			Expect(last.AuthMode).To(Equal(acceptanceharness.AuthModeNone))
			Expect(last.Method).To(Equal(http.MethodPost))
			Expect(last.FixtureID).To(Equal("installation-fixture"))
		})

		Context("when an OAuth access token is misused against this App-level endpoint", func() {
			It("rejects it and records AuthModeRejected, rather than treating it as an unverifiable App JWT", func() {
				misuseFx := fakegithub.NewFixture("installation-misuse-fixture")
				misuseFx.OAuth.ClientID = "test-client-id"
				misuseFx.OAuth.ClientSecret = "test-client-secret"
				misuseFx.OAuth.Tokens["oauth-token-xyz"] = fakegithub.OAuthTokenEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioOK}
				misuseFx.Installation.Installations[123] = fakegithub.InstallationEntry{Token: "installation-token-abc", Scenario: fakegithub.ScenarioOK}

				misuseServer := fakegithub.NewServer(&misuseFx)
				defer misuseServer.Close()

				req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v3/app/installations/%d/access_tokens", misuseServer.URL(), 123), nil)
				Expect(err).NotTo(HaveOccurred())
				req.Header.Set("Authorization", "token oauth-token-xyz")

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

	Describe("GET /api/v3/repos/{owner}/{repo}/installation", func() {
		resolve := func(ownerRepo string) *http.Response {
			resp, err := http.Get(fmt.Sprintf("%s/api/v3/repos/%s/installation", server.URL(), ownerRepo))
			Expect(err).NotTo(HaveOccurred())
			return resp
		}

		It("resolves a mapped repo to its installation ID", func() {
			resp := resolve("acme/widgets")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var body struct {
				ID int64 `json:"id"`
			}
			Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
			Expect(body.ID).To(Equal(int64(123)))
		})

		It("returns 404 for a repo the fixture never mapped", func() {
			resp := resolve("acme/unmapped-repo")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("models ScenarioAuthFail for a repo mapped with auth-failure", func() {
			resp := resolve("acme/authfail-repo")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("models ScenarioTransient for a repo mapped with transient", func() {
			resp := resolve("acme/transient-repo")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusServiceUnavailable))
		})

		It("models ScenarioNotFound for a repo explicitly mapped as not-found", func() {
			resp := resolve("acme/notfound-repo")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("records the resolution with AuthModeNone", func() {
			resp := resolve("acme/widgets")
			resp.Body.Close()

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			Expect(records[len(records)-1].AuthMode).To(Equal(acceptanceharness.AuthModeNone))
		})

		Context("when an OAuth access token is misused against this repo-installation-resolution endpoint", func() {
			It("rejects it and records AuthModeRejected, rather than treating it as an unverifiable App JWT", func() {
				misuseFx := fakegithub.NewFixture("installation-misuse-fixture")
				misuseFx.OAuth.ClientID = "test-client-id"
				misuseFx.OAuth.ClientSecret = "test-client-secret"
				misuseFx.OAuth.Tokens["oauth-token-xyz"] = fakegithub.OAuthTokenEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioOK}
				misuseFx.Installation.Installations[123] = fakegithub.InstallationEntry{Token: "installation-token-abc", Scenario: fakegithub.ScenarioOK}
				misuseFx.Installation.RepoMappings["acme/widgets"] = fakegithub.RepoInstallationEntry{InstallationID: 123, Scenario: fakegithub.ScenarioOK}

				misuseServer := fakegithub.NewServer(&misuseFx)
				defer misuseServer.Close()

				req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v3/repos/acme/widgets/installation", misuseServer.URL()), nil)
				Expect(err).NotTo(HaveOccurred())
				req.Header.Set("Authorization", "token oauth-token-xyz")

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

	Describe("GET /api/v3/repos/{owner}/{repo}/collaborators/{username}/permission", func() {
		checkPermission := func(ownerRepo, username, authHeaderValue string) *http.Response {
			req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v3/repos/%s/collaborators/%s/permission", server.URL(), ownerRepo, username), nil)
			Expect(err).NotTo(HaveOccurred())
			req.Header.Set("Authorization", authHeaderValue)
			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			return resp
		}

		It("returns the fixture's registered permission level for a known collaborator, authenticated with an installation token", func() {
			resp := checkPermission("acme/widgets", "octocat", "token installation-token-abc")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			var body struct {
				Permission string `json:"permission"`
			}
			Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
			Expect(body.Permission).To(Equal("write"))
		})

		It("returns 404 for a collaborator the fixture never registered", func() {
			resp := checkPermission("acme/widgets", "ghost-user", "token installation-token-abc")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("models ScenarioAuthFail for a collaborator registered as auth-failure", func() {
			resp := checkPermission("acme/widgets", "authfail-user", "token installation-token-abc")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
		})

		It("models ScenarioTransient for a collaborator registered as transient", func() {
			resp := checkPermission("acme/widgets", "transient-user", "token installation-token-abc")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusServiceUnavailable))
		})

		It("models ScenarioNotFound for a collaborator explicitly registered as not-found", func() {
			resp := checkPermission("acme/widgets", "notfound-user", "token installation-token-abc")
			defer resp.Body.Close()
			Expect(resp.StatusCode).To(Equal(http.StatusNotFound))
		})

		It("records a successful check with AuthModeInstallation", func() {
			resp := checkPermission("acme/widgets", "octocat", "token installation-token-abc")
			resp.Body.Close()

			records := server.Recorder().Records()
			Expect(records).NotTo(BeEmpty())
			last := records[len(records)-1]
			Expect(last.AuthMode).To(Equal(acceptanceharness.AuthModeInstallation))
			Expect(last.Method).To(Equal(http.MethodGet))
		})

		Context("when an OAuth access token is misused against this repo endpoint", func() {
			It("rejects it and records AuthModeRejected, rather than treating it as an installation credential", func() {
				misuseFx := fakegithub.NewFixture("installation-misuse-fixture")
				misuseFx.OAuth.ClientID = "test-client-id"
				misuseFx.OAuth.ClientSecret = "test-client-secret"
				misuseFx.OAuth.Tokens["oauth-token-xyz"] = fakegithub.OAuthTokenEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioOK}
				misuseFx.Installation.Installations[123] = fakegithub.InstallationEntry{Token: "installation-token-abc", Scenario: fakegithub.ScenarioOK}
				misuseFx.Installation.RepoMappings["acme/widgets"] = fakegithub.RepoInstallationEntry{InstallationID: 123, Scenario: fakegithub.ScenarioOK}
				misuseFx.Installation.Permissions["acme/widgets/octocat"] = fakegithub.PermissionEntry{Level: "write", Scenario: fakegithub.ScenarioOK}

				misuseServer := fakegithub.NewServer(&misuseFx)
				defer misuseServer.Close()

				req, err := http.NewRequest(http.MethodGet, misuseServer.URL()+"/api/v3/repos/acme/widgets/collaborators/octocat/permission", nil)
				Expect(err).NotTo(HaveOccurred())
				req.Header.Set("Authorization", "token oauth-token-xyz")

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
