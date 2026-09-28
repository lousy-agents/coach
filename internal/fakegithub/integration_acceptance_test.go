package fakegithub_test

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/acceptanceharness"
	"github.com/lousy-agents/coach/internal/fakegithub"
)

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
})
