package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/authn"
	"github.com/lousy-agents/coach/internal/authz"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
	"github.com/lousy-agents/coach/internal/fakegithub"
)

const (
	mainTestSigningKey = "test-signing-secret-at-least-32-bytes!!"
	mainTestIssuer     = "https://coach-api.test"
)

// stubRepoAuthorizer is a minimal authz.RepoAuthorizer test double: it
// always allows, since this suite's job is proving the composed handler
// wires authn + coachapi together, not re-testing ADR-003's authorization
// matrix (that belongs to internal/authz and internal/coachapi).
type stubRepoAuthorizer struct{}

func (stubRepoAuthorizer) Authorize(context.Context, string, string, string) error { return nil }

var _ authz.RepoAuthorizer = stubRepoAuthorizer{}

// stubTaskQueue is a minimal queue.TaskQueue test double; only Enqueue has
// meaningful behavior (recording calls), matching this suite's goal of
// proving the composed handler enqueues rather than re-testing TaskQueue
// semantics owned by internal/coachapi/queue.
type stubTaskQueue struct {
	mu       sync.Mutex
	enqueued []queue.Task
}

func (q *stubTaskQueue) Enqueue(_ context.Context, task queue.Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.enqueued = append(q.enqueued, task)
	return nil
}

func (q *stubTaskQueue) Claim(context.Context) (queue.Claim, bool, error) {
	return queue.Claim{}, false, nil
}

func (q *stubTaskQueue) Complete(context.Context, queue.Claim) error { return nil }

func (q *stubTaskQueue) Nack(context.Context, queue.Claim, bool) error { return nil }

var _ queue.TaskQueue = (*stubTaskQueue)(nil)

func newTestDependencies() Dependencies {
	return Dependencies{
		Store:      coachapi.NewMemoryStore(),
		Authorizer: stubRepoAuthorizer{},
		Queue:      &stubTaskQueue{},
	}
}

func doRequest(h http.Handler, method, path, bearer string, body []byte) (int, []byte) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.Bytes()
}

func mintTestToken(h http.Handler) string {
	code, body := doRequest(h, http.MethodPost, "/v1/auth/test-mint", "", []byte(`{"subject":"1","login":"octocat"}`))
	Expect(code).To(Equal(http.StatusOK), "test-mint body=%s", body)
	var resp struct {
		Token string `json:"token"`
	}
	Expect(json.Unmarshal(body, &resp)).To(Succeed())
	Expect(resp.Token).NotTo(BeEmpty())
	return resp.Token
}

var _ = Describe("cmd/coach-api composed handler", func() {
	When("the handler is composed with test-mint enabled and no GitHub OAuth configured", func() {
		var handler http.Handler

		BeforeEach(func() {
			cfg := Config{
				JWTSigningKey:       []byte(mainTestSigningKey),
				JWTIssuer:           mainTestIssuer,
				JWTTokenTTL:         time.Hour,
				AuthTestMintEnabled: true,
			}
			h, err := buildHandler(cfg, newTestDependencies())
			Expect(err).NotTo(HaveOccurred())
			handler = h
		})

		It("mints a token, creates a job, reads it back, and serves /v1/me through one composed handler", func() {
			token := mintTestToken(handler)

			code, body := doRequest(handler, http.MethodGet, "/v1/me", token, nil)
			Expect(code).To(Equal(http.StatusOK), "body=%s", body)
			var principal coachapi.Principal
			Expect(json.Unmarshal(body, &principal)).To(Succeed())
			Expect(principal.Login).To(Equal("octocat"))

			createBody := []byte(`{"kind":"repo_baseline_scan","params":{"repo_owner":"acme","repo_name":"widgets"}}`)
			code, body = doRequest(handler, http.MethodPost, "/v1/jobs", token, createBody)
			Expect(code).To(Equal(http.StatusAccepted), "body=%s", body)
			var created coachapi.CreateJobResponse
			Expect(json.Unmarshal(body, &created)).To(Succeed())
			Expect(created.ID).NotTo(BeEmpty())

			code, body = doRequest(handler, http.MethodGet, "/v1/jobs/"+created.ID, token, nil)
			Expect(code).To(Equal(http.StatusOK), "body=%s", body)
			var status coachapi.JobStatusResponse
			Expect(json.Unmarshal(body, &status)).To(Succeed())
			Expect(status.ID).To(Equal(created.ID))
		})

		It("rejects an unauthenticated POST /v1/jobs with 401 through the composed handler", func() {
			createBody := []byte(`{"kind":"repo_baseline_scan","params":{"repo_owner":"acme","repo_name":"widgets"}}`)
			code, body := doRequest(handler, http.MethodPost, "/v1/jobs", "", createBody)
			Expect(code).To(Equal(http.StatusUnauthorized), "body=%s", body)
			var env coachapi.ErrorEnvelope
			Expect(json.Unmarshal(body, &env)).To(Succeed())
			Expect(env.Error.Code).To(Equal(coachapi.ErrorCodeUnauthenticated))
		})

		It("returns 404 for the OAuth routes since GitHubOAuth is not configured", func() {
			code, _ := doRequest(handler, http.MethodGet, "/oauth/github/start", "", nil)
			Expect(code).To(Equal(http.StatusNotFound))
		})
	})

	When("the handler is composed with GitHub OAuth configured", func() {
		It("serves GET /oauth/github/start as a redirect to the configured GitHub OAuth authorize endpoint", func() {
			fx := fakegithub.NewFixture("coach-api-main-acceptance")
			fx.OAuth.ClientID = "coach-oauth-client-id"
			fx.OAuth.ClientSecret = "coach-oauth-client-secret"
			srv := fakegithub.NewServer(&fx)
			DeferCleanup(srv.Close)

			cfg := Config{
				JWTSigningKey: []byte(mainTestSigningKey),
				JWTIssuer:     mainTestIssuer,
				JWTTokenTTL:   time.Hour,
				GitHubOAuth: &authn.GitHubOAuthConfig{
					ClientID:     fx.OAuth.ClientID,
					ClientSecret: fx.OAuth.ClientSecret,
					BaseURL:      srv.URL(),
					RedirectURI:  "http://coach.test/oauth/github/callback",
				},
			}
			handler, err := buildHandler(cfg, newTestDependencies())
			Expect(err).NotTo(HaveOccurred())

			req := httptest.NewRequest(http.MethodGet, "/oauth/github/start", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			Expect(rec.Code).To(Equal(http.StatusFound), "body=%s", rec.Body.String())
			Expect(rec.Header().Get("Location")).To(ContainSubstring(srv.URL()))
		})
	})
})
