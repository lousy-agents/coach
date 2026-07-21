package fakegithub_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/fakegithub"
)

var _ = Describe("fakegithub.NewServer", func() {
	It("panics immediately with a message identifying the nil Fixture, rather than deferring the failure to first request", func() {
		Expect(func() { fakegithub.NewServer(nil) }).To(PanicWith(ContainSubstring("nil Fixture")))
	})
})

var _ = Describe("fakegithub.Handler", func() {
	It("panics immediately with a message identifying the nil Fixture, rather than deferring the failure to first request", func() {
		Expect(func() { fakegithub.Handler(nil) }).To(PanicWith(ContainSubstring("nil Fixture")))
	})

	It("serves a full request on its own, without going through NewServer, and records it via the returned Recorder", func() {
		fx := fakegithub.NewFixture("handler-fixture")
		fx.OAuth.Identities["octocat"] = fakegithub.Identity{ID: 1, Login: "octocat"}
		fx.OAuth.Tokens["token-ok"] = fakegithub.OAuthTokenEntry{IdentityLogin: "octocat", Scenario: fakegithub.ScenarioOK}

		handler, recorder := fakegithub.Handler(&fx)
		server := httptest.NewServer(handler)
		defer server.Close()

		req, err := http.NewRequest(http.MethodGet, server.URL+"/api/v3/user", nil)
		Expect(err).NotTo(HaveOccurred())
		req.Header.Set("Authorization", "Bearer token-ok")

		resp, err := http.DefaultClient.Do(req)
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))

		var body struct {
			ID    int64  `json:"id"`
			Login string `json:"login"`
		}
		Expect(json.NewDecoder(resp.Body).Decode(&body)).To(Succeed())
		Expect(body.Login).To(Equal("octocat"))

		records := recorder.Records()
		Expect(records).To(HaveLen(1))
		Expect(records[0].FixtureID).To(Equal("handler-fixture"))
		Expect(records[0].AuthMode).To(Equal(acceptanceharness.AuthModeOAuth))
	})
})
