package fakegithub_test

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/fakegithub"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("recorder sequence across all five endpoint families", func() {
	var server *fakegithub.Server

	BeforeEach(func() {
		server = fakegithub.NewServer(newIntegrationFixture())
	})

	AfterEach(func() {
		server.Close()
	})

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
			acceptanceharness.AuthModeNone,
			acceptanceharness.AuthModeNone,
			acceptanceharness.AuthModeNone,
			acceptanceharness.AuthModeNone,
			acceptanceharness.AuthModeOAuth,
			acceptanceharness.AuthModeInstallation,
			acceptanceharness.AuthModeInstallation,
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

var _ = Describe("concurrent requests against one Server", func() {
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

		results := make([]concurrentOAuthOutcome, concurrency)

		var wg sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				results[i] = runConcurrentOAuthCycle(server, i)
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

		Expect(fx.OAuth.Tokens).To(HaveLen(concurrency + 1))
		for i := 0; i < concurrency; i++ {
			Expect(fx.OAuth.Codes).NotTo(HaveKey(fmt.Sprintf("concurrent-code-%d", i)))
		}
	})
})
