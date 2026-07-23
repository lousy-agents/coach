package coachapi_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/authn"
	"github.com/lousy-agents/coach/internal/authz"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
	"github.com/lousy-agents/coach/internal/fakegithub"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

const (
	serverTestIssuer = "https://coach-api.test"
	serverTestSecret = "test-signing-secret-at-least-32-bytes!!"
)

func serverFixedNow(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

func newAuthnServiceForServer(opts authn.Options) *authn.Service {
	if opts.SigningKey == nil {
		opts.SigningKey = []byte(serverTestSecret)
	}
	if opts.Issuer == "" {
		opts.Issuer = serverTestIssuer
	}
	if opts.TokenTTL == 0 {
		opts.TokenTTL = time.Hour
	}
	if opts.Now == nil {
		opts.Now = serverFixedNow(time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC))
	}
	if opts.Denylist == nil {
		opts.Denylist = authn.NewMemoryDenylist()
	}
	svc, err := authn.New(opts)
	Expect(err).NotTo(HaveOccurred())
	return svc
}

func mustIssueToken(svc *authn.Service, p coachapi.Principal) string {
	tok, err := svc.Issue(context.Background(), p)
	Expect(err).NotTo(HaveOccurred())
	return tok
}

func decodeServerEnvelope(body []byte) coachapi.ErrorEnvelope {
	var env coachapi.ErrorEnvelope
	Expect(json.Unmarshal(body, &env)).To(Succeed(), "body=%s", body)
	return env
}

func doServerReq(h http.Handler, method, path, bearer string, body []byte) (int, []byte) {
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

func expectEnvelope(code int, body []byte, wantStatus int, wantCode string) coachapi.ErrorEnvelope {
	Expect(code).To(Equal(wantStatus), "body=%s", body)
	env := decodeServerEnvelope(body)
	Expect(env.Error.Code).To(Equal(wantCode))
	Expect(strings.TrimSpace(env.Error.Message)).NotTo(BeEmpty())
	return env
}

// stubRepoAuthorizer records every Authorize call and returns a configured
// error (or nil) for every call.
type stubRepoAuthorizer struct {
	mu    sync.Mutex
	err   error
	calls []stubAuthorizeCall
}

type stubAuthorizeCall struct {
	login, owner, repo string
}

func (s *stubRepoAuthorizer) Authorize(_ context.Context, login, owner, repo string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, stubAuthorizeCall{login: login, owner: owner, repo: repo})
	return s.err
}

func (s *stubRepoAuthorizer) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

var _ authz.RepoAuthorizer = (*stubRepoAuthorizer)(nil)

// stubTaskQueue is a hand-rolled queue.TaskQueue test double; only Enqueue
// has real (configurable) behavior, matching the epic's guidance that Claim/
// Complete/Nack can be simple stubs for these HTTP-layer tests.
type stubTaskQueue struct {
	mu         sync.Mutex
	enqueueErr error
	enqueued   []queue.Task
}

func (q *stubTaskQueue) Enqueue(_ context.Context, task queue.Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.enqueueErr != nil {
		return q.enqueueErr
	}
	q.enqueued = append(q.enqueued, task)
	return nil
}

func (q *stubTaskQueue) Claim(context.Context) (queue.Claim, bool, error) {
	return queue.Claim{}, false, nil
}

func (q *stubTaskQueue) Complete(context.Context, queue.Claim) error { return nil }

func (q *stubTaskQueue) Nack(context.Context, queue.Claim, bool) error { return nil }

func (q *stubTaskQueue) enqueuedTasks() []queue.Task {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := make([]queue.Task, len(q.enqueued))
	copy(out, q.enqueued)
	return out
}

var _ queue.TaskQueue = (*stubTaskQueue)(nil)

// spyJobStore wraps a MemoryStore and counts CreateJob invocations so tests
// can prove a rejected submit persisted nothing.
type spyJobStore struct {
	*coachapi.MemoryStore
	mu          sync.Mutex
	createCalls int
}

func newSpyJobStore() *spyJobStore {
	return &spyJobStore{MemoryStore: coachapi.NewMemoryStore()}
}

func (s *spyJobStore) CreateJob(ctx context.Context, job coachapi.Job) error {
	s.mu.Lock()
	s.createCalls++
	s.mu.Unlock()
	return s.MemoryStore.CreateJob(ctx, job)
}

func (s *spyJobStore) createJobCalls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createCalls
}

func sequentialJobIDs(prefix string) func() string {
	n := 0
	return func() string {
		n++
		return fmt.Sprintf("%s-%d", prefix, n)
	}
}

func principalAlice() coachapi.Principal {
	return coachapi.Principal{Provider: "github", Subject: "1001", Login: "alice"}
}

func principalBob() coachapi.Principal {
	return coachapi.Principal{Provider: "github", Subject: "2002", Login: "bob"}
}

func validRepoBaselineScanBody() []byte {
	return []byte(`{"kind":"repo_baseline_scan","params":{"repo_owner":"acme","repo_name":"widgets"}}`)
}

func newTestServer(store coachapi.JobStore, az authz.RepoAuthorizer, q queue.TaskQueue, now func() time.Time, newJobID func() string) *coachapi.Server {
	srv, err := coachapi.NewServer(coachapi.ServerConfig{
		Store:      store,
		Authorizer: az,
		Queue:      q,
		Now:        now,
		NewJobID:   newJobID,
	})
	Expect(err).NotTo(HaveOccurred())
	return srv
}

var _ = Describe("coachapi.Server HTTP surface (POST /v1/jobs, GET /v1/jobs/{id}, GET /v1/jobs/{id}/report)", func() {
	var (
		authnSvc *authn.Service
	)

	BeforeEach(func() {
		authnSvc = newAuthnServiceForServer(authn.Options{})
	})

	When("an authenticated principal submits a valid repo_baseline_scan job and a nil-erroring authorizer allows it", func() {
		It("returns 202 with an id, GET status shows queued, and the queue records a versioned payload with the job id", func() {
			store := newSpyJobStore()
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)), sequentialJobIDs("happy"))
			h := authnSvc.Middleware(srv.Handler())

			tok := mustIssueToken(authnSvc, principalAlice())
			code, body := doServerReq(h, http.MethodPost, "/v1/jobs", tok, validRepoBaselineScanBody())
			Expect(code).To(Equal(http.StatusAccepted), "body=%s", body)
			var created coachapi.CreateJobResponse
			Expect(json.Unmarshal(body, &created)).To(Succeed())
			Expect(created.ID).NotTo(BeEmpty())

			code, body = doServerReq(h, http.MethodGet, "/v1/jobs/"+created.ID, tok, nil)
			Expect(code).To(Equal(http.StatusOK), "body=%s", body)
			var status coachapi.JobStatusResponse
			Expect(json.Unmarshal(body, &status)).To(Succeed())
			Expect(status.ID).To(Equal(created.ID))
			Expect(status.Status).To(Equal(coachapi.JobStatusQueued))
			Expect(status.ReportURL).To(BeEmpty(), "report_url must be empty until completed")

			tasks := q.enqueuedTasks()
			Expect(tasks).To(HaveLen(1))
			Expect(tasks[0].ID).To(Equal(created.ID), "task ID is the job idempotency key")
			var payload map[string]any
			Expect(json.Unmarshal(tasks[0].Payload, &payload)).To(Succeed(), "payload=%s", tasks[0].Payload)
			Expect(payload).To(Equal(map[string]any{
				"schema_version": float64(1),
				"job_id":         created.ID,
			}), "ADR-006 requires versioned queue payloads; exact enqueued JSON shape")

			Expect(az.callCount()).To(Equal(1))
		})
	})

	When("an authenticated principal submits repo_owner/repo_name with surrounding whitespace", func() {
		It("returns 202 and persists params whose owner/repo exactly match the canonical identifiers used for authorization", func() {
			store := newSpyJobStore()
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)), sequentialJobIDs("pad"))
			h := authnSvc.Middleware(srv.Handler())

			tok := mustIssueToken(authnSvc, principalAlice())
			// Padded identifiers authorize as acme/widgets; the worker must
			// receive the same canonical pair, not the raw padded JSON.
			paddedBody := []byte(`{"kind":"repo_baseline_scan","params":{"repo_owner":" acme ","repo_name":" widgets ","ref":"main"}}`)
			code, body := doServerReq(h, http.MethodPost, "/v1/jobs", tok, paddedBody)
			Expect(code).To(Equal(http.StatusAccepted), "body=%s", body)
			var created coachapi.CreateJobResponse
			Expect(json.Unmarshal(body, &created)).To(Succeed())

			Expect(az.callCount()).To(Equal(1))
			Expect(az.calls[0].owner).To(Equal("acme"))
			Expect(az.calls[0].repo).To(Equal("widgets"))

			job, err := store.GetJob(context.Background(), created.ID)
			Expect(err).NotTo(HaveOccurred())
			var stored coachapi.RepoBaselineScanParams
			Expect(json.Unmarshal(job.Params, &stored)).To(Succeed(), "params=%s", job.Params)
			Expect(stored.RepoOwner).To(Equal("acme"), "persisted owner must match the authorized canonical owner")
			Expect(stored.RepoName).To(Equal("widgets"), "persisted name must match the authorized canonical name")
			Expect(stored.Ref).To(Equal("main"), "intended ref semantics must be preserved")
			// Reject any residual padding in the raw stored JSON.
			Expect(string(job.Params)).NotTo(ContainSubstring(`" acme "`))
			Expect(string(job.Params)).NotTo(ContainSubstring(`" widgets "`))
		})
	})

	Describe("400 invalid_request", func() {
		var (
			store *spyJobStore
			az    *stubRepoAuthorizer
			q     *stubTaskQueue
			h     http.Handler
			tok   string
		)
		BeforeEach(func() {
			store = newSpyJobStore()
			az = &stubRepoAuthorizer{}
			q = &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("inv"))
			h = authnSvc.Middleware(srv.Handler())
			tok = mustIssueToken(authnSvc, principalAlice())
		})

		It("rejects malformed JSON body", func() {
			code, body := doServerReq(h, http.MethodPost, "/v1/jobs", tok, []byte(`{not-json`))
			expectEnvelope(code, body, http.StatusBadRequest, coachapi.ErrorCodeInvalidRequest)
			Expect(store.createJobCalls()).To(Equal(0))
		})

		It("rejects a body missing repo_owner/repo_name", func() {
			body := []byte(`{"kind":"repo_baseline_scan","params":{"ref":"main"}}`)
			code, respBody := doServerReq(h, http.MethodPost, "/v1/jobs", tok, body)
			expectEnvelope(code, respBody, http.StatusBadRequest, coachapi.ErrorCodeInvalidRequest)
			Expect(store.createJobCalls()).To(Equal(0))
		})

		It("rejects a git_url key present in params (DisallowUnknownFields rejection)", func() {
			body := []byte(`{"kind":"repo_baseline_scan","params":{"repo_owner":"acme","repo_name":"widgets","git_url":"https://evil.example/x.git"}}`)
			code, respBody := doServerReq(h, http.MethodPost, "/v1/jobs", tok, body)
			expectEnvelope(code, respBody, http.StatusBadRequest, coachapi.ErrorCodeInvalidRequest)
			Expect(store.createJobCalls()).To(Equal(0))
		})
	})

	When("the request names an unrecognized job kind", func() {
		It("returns 400 unsupported_job_kind and persists nothing", func() {
			store := newSpyJobStore()
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("kind"))
			h := authnSvc.Middleware(srv.Handler())
			tok := mustIssueToken(authnSvc, principalAlice())

			body := []byte(`{"kind":"delete_universe","params":{}}`)
			code, respBody := doServerReq(h, http.MethodPost, "/v1/jobs", tok, body)
			expectEnvelope(code, respBody, http.StatusBadRequest, coachapi.ErrorCodeUnsupportedJobKind)
			Expect(store.createJobCalls()).To(Equal(0))
		})
	})

	Describe("401 unauthenticated on every route when no bearer token is presented", func() {
		var (
			h  http.Handler
			id string
		)
		BeforeEach(func() {
			store := coachapi.NewMemoryStore()
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("noauth"))
			h = authnSvc.Middleware(srv.Handler())
			id = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		})

		It("rejects POST /v1/jobs", func() {
			code, body := doServerReq(h, http.MethodPost, "/v1/jobs", "", validRepoBaselineScanBody())
			expectEnvelope(code, body, http.StatusUnauthorized, coachapi.ErrorCodeUnauthenticated)
		})

		It("rejects GET /v1/jobs/{id}", func() {
			code, body := doServerReq(h, http.MethodGet, "/v1/jobs/"+id, "", nil)
			expectEnvelope(code, body, http.StatusUnauthorized, coachapi.ErrorCodeUnauthenticated)
		})

		It("rejects GET /v1/jobs/{id}/report", func() {
			code, body := doServerReq(h, http.MethodGet, "/v1/jobs/"+id+"/report", "", nil)
			expectEnvelope(code, body, http.StatusUnauthorized, coachapi.ErrorCodeUnauthenticated)
		})
	})

	When("the configured RepoAuthorizer reports the principal is not authorized for the repo", func() {
		It("returns 403 repo_not_authorized with an actionable message and persists nothing", func() {
			store := newSpyJobStore()
			az := &stubRepoAuthorizer{err: fmt.Errorf("stub: no role: %w", authz.ErrNotAuthorized)}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("notauthz"))
			h := authnSvc.Middleware(srv.Handler())
			tok := mustIssueToken(authnSvc, principalAlice())

			code, body := doServerReq(h, http.MethodPost, "/v1/jobs", tok, validRepoBaselineScanBody())
			env := expectEnvelope(code, body, http.StatusForbidden, coachapi.ErrorCodeRepoNotAuthorized)
			Expect(env.Error.Message).To(Or(ContainSubstring("no role"), ContainSubstring("not authorized"), ContainSubstring("denied")),
				"message must be actionable, not a bare 'forbidden'")
			Expect(store.createJobCalls()).To(Equal(0))
		})
	})

	When("the live GitHubRepoAuthorizer sees an unrecognized effective permission for the principal", func() {
		It("returns 403 repo_not_authorized and persists nothing (fail closed on unknown permission)", func() {
			fx := fakegithub.NewFixture("server-unknown-perm")
			fx.Installation.Installations[1] = fakegithub.InstallationEntry{Token: "server-install-token", Scenario: fakegithub.ScenarioOK}
			fx.Installation.RepoMappings["acme/widgets"] = fakegithub.RepoInstallationEntry{InstallationID: 1, Scenario: fakegithub.ScenarioOK}
			fx.Installation.Permissions["acme/widgets/alice"] = fakegithub.PermissionEntry{Level: "superadmin", Scenario: fakegithub.ScenarioOK}
			ghServer := fakegithub.NewServer(&fx)
			DeferCleanup(ghServer.Close)

			key, err := rsa.GenerateKey(rand.Reader, 2048)
			Expect(err).NotTo(HaveOccurred())
			privateKey := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
			credentials, err := githubingest.NewCredentialResolver(githubingest.CredentialResolverConfig{
				AppID: 12345, PrivateKey: privateKey, BaseURL: ghServer.URL(),
			})
			Expect(err).NotTo(HaveOccurred())
			az, err := authz.NewGitHubRepoAuthorizer(authz.GitHubRepoAuthorizerConfig{
				Credentials: credentials, BaseURL: ghServer.URL(),
			})
			Expect(err).NotTo(HaveOccurred())

			store := newSpyJobStore()
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("unkperm"))
			h := authnSvc.Middleware(srv.Handler())
			tok := mustIssueToken(authnSvc, principalAlice())

			code, body := doServerReq(h, http.MethodPost, "/v1/jobs", tok, validRepoBaselineScanBody())
			expectEnvelope(code, body, http.StatusForbidden, coachapi.ErrorCodeRepoNotAuthorized)
			Expect(store.createJobCalls()).To(Equal(0))
			Expect(q.enqueuedTasks()).To(BeEmpty())
		})
	})

	When("the configured RepoAuthorizer fails transiently (not ErrNotAuthorized)", func() {
		It("returns 503 internal_error and persists nothing", func() {
			store := newSpyJobStore()
			az := &stubRepoAuthorizer{err: errors.New("github: transient failure")}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("transient"))
			h := authnSvc.Middleware(srv.Handler())
			tok := mustIssueToken(authnSvc, principalAlice())

			code, body := doServerReq(h, http.MethodPost, "/v1/jobs", tok, validRepoBaselineScanBody())
			expectEnvelope(code, body, http.StatusServiceUnavailable, coachapi.ErrorCodeInternalError)
			Expect(store.createJobCalls()).To(Equal(0))
		})
	})

	Describe("404 job_not_found", func() {
		var (
			h         http.Handler
			tok       string
			unknownID string
		)
		BeforeEach(func() {
			store := coachapi.NewMemoryStore()
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("notfound"))
			h = authnSvc.Middleware(srv.Handler())
			tok = mustIssueToken(authnSvc, principalAlice())
			unknownID = "ffffffff-ffff-ffff-ffff-ffffffffffff"
		})

		It("returns 404 for GET /v1/jobs/{unknown-id}", func() {
			code, body := doServerReq(h, http.MethodGet, "/v1/jobs/"+unknownID, tok, nil)
			expectEnvelope(code, body, http.StatusNotFound, coachapi.ErrorCodeJobNotFound)
		})

		It("returns 404 for GET /v1/jobs/{unknown-id}/report", func() {
			code, body := doServerReq(h, http.MethodGet, "/v1/jobs/"+unknownID+"/report", tok, nil)
			expectEnvelope(code, body, http.StatusNotFound, coachapi.ErrorCodeJobNotFound)
		})
	})

	When("a different principal reads an incomplete (queued) job's status", func() {
		It("returns 403 unauthorized -- ownership precedes status, even before completion", func() {
			store := coachapi.NewMemoryStore()
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("cross"))
			h := authnSvc.Middleware(srv.Handler())

			owner := principalAlice()
			job := coachapi.Job{
				ID: "cross-status-1", Kind: coachapi.JobKindRepoBaselineScan,
				Params: json.RawMessage(`{"repo_owner":"acme","repo_name":"widgets"}`),
				Status: coachapi.JobStatusQueued, CreatedAt: time.Now(),
				CreatedByProvider: owner.Provider, CreatedBySubject: owner.Subject, CreatedByLogin: owner.Login,
			}
			Expect(store.CreateJob(context.Background(), job)).To(Succeed())

			bobTok := mustIssueToken(authnSvc, principalBob())
			code, body := doServerReq(h, http.MethodGet, "/v1/jobs/"+job.ID, bobTok, nil)
			expectEnvelope(code, body, http.StatusForbidden, coachapi.ErrorCodeUnauthorized)
		})
	})

	When("a different principal reads an incomplete job's report", func() {
		It("returns 403 unauthorized, not 409, for a queued job", func() {
			store := coachapi.NewMemoryStore()
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("crossreport"))
			h := authnSvc.Middleware(srv.Handler())

			owner := principalAlice()
			job := coachapi.Job{
				ID: "cross-report-1", Kind: coachapi.JobKindRepoBaselineScan,
				Params: json.RawMessage(`{"repo_owner":"acme","repo_name":"widgets"}`),
				Status: coachapi.JobStatusQueued, CreatedAt: time.Now(),
				CreatedByProvider: owner.Provider, CreatedBySubject: owner.Subject, CreatedByLogin: owner.Login,
			}
			Expect(store.CreateJob(context.Background(), job)).To(Succeed())

			bobTok := mustIssueToken(authnSvc, principalBob())
			code, body := doServerReq(h, http.MethodGet, "/v1/jobs/"+job.ID+"/report", bobTok, nil)
			expectEnvelope(code, body, http.StatusForbidden, coachapi.ErrorCodeUnauthorized)
		})
	})

	DescribeTable("409 job_not_completed for the owning principal, message mentions the actual status",
		func(seed func(store *coachapi.MemoryStore, jobID string, owner coachapi.Principal), wantStatusInMessage string) {
			store := coachapi.NewMemoryStore()
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("notcompleted"))
			h := authnSvc.Middleware(srv.Handler())

			owner := principalAlice()
			jobID := "not-completed-" + wantStatusInMessage
			seed(store, jobID, owner)

			tok := mustIssueToken(authnSvc, owner)
			code, body := doServerReq(h, http.MethodGet, "/v1/jobs/"+jobID+"/report", tok, nil)
			env := expectEnvelope(code, body, http.StatusConflict, coachapi.ErrorCodeJobNotCompleted)
			Expect(env.Error.Message).To(ContainSubstring(wantStatusInMessage))
		},
		Entry("queued", func(store *coachapi.MemoryStore, jobID string, owner coachapi.Principal) {
			Expect(store.CreateJob(context.Background(), coachapi.Job{
				ID: jobID, Kind: coachapi.JobKindRepoBaselineScan,
				Params: json.RawMessage(`{"repo_owner":"acme","repo_name":"widgets"}`),
				Status: coachapi.JobStatusQueued, CreatedAt: time.Now(),
				CreatedByProvider: owner.Provider, CreatedBySubject: owner.Subject, CreatedByLogin: owner.Login,
			})).To(Succeed())
		}, "queued"),
		Entry("running", func(store *coachapi.MemoryStore, jobID string, owner coachapi.Principal) {
			Expect(store.CreateJob(context.Background(), coachapi.Job{
				ID: jobID, Kind: coachapi.JobKindRepoBaselineScan,
				Params: json.RawMessage(`{"repo_owner":"acme","repo_name":"widgets"}`),
				Status: coachapi.JobStatusRunning, CreatedAt: time.Now(),
				CreatedByProvider: owner.Provider, CreatedBySubject: owner.Subject, CreatedByLogin: owner.Login,
			})).To(Succeed())
		}, "running"),
		Entry("failed", func(store *coachapi.MemoryStore, jobID string, owner coachapi.Principal) {
			Expect(store.CreateJob(context.Background(), coachapi.Job{
				ID: jobID, Kind: coachapi.JobKindRepoBaselineScan,
				Params: json.RawMessage(`{"repo_owner":"acme","repo_name":"widgets"}`),
				Status: coachapi.JobStatusQueued, CreatedAt: time.Now(),
				CreatedByProvider: owner.Provider, CreatedBySubject: owner.Subject, CreatedByLogin: owner.Login,
			})).To(Succeed())
			Expect(store.RecordFailure(context.Background(), jobID, "clone failed: timeout", time.Now())).To(Succeed())
		}, "failed"),
	)

	When("the owning principal reads a completed job's report", func() {
		It("returns 200 with the assembled Report JSON", func() {
			store := coachapi.NewMemoryStore()
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("completed"))
			h := authnSvc.Middleware(srv.Handler())

			owner := principalAlice()
			jobID := "completed-job-1"
			Expect(store.CreateJob(context.Background(), coachapi.Job{
				ID: jobID, Kind: coachapi.JobKindRepoBaselineScan,
				Params: json.RawMessage(`{"repo_owner":"acme","repo_name":"widgets"}`),
				Status: coachapi.JobStatusQueued, CreatedAt: time.Now(),
				CreatedByProvider: owner.Provider, CreatedBySubject: owner.Subject, CreatedByLogin: owner.Login,
			})).To(Succeed())

			generatedAt := time.Date(2026, 7, 23, 13, 0, 0, 0, time.UTC)
			Expect(store.RecordCompletion(context.Background(), jobID, coachapi.Completion{
				Attempt:     1,
				CommitSHA:   "abc123def4567890abc123def4567890abc123de",
				Versions:    coachapi.ReportVersions{Analyzer: "codesignal@1"},
				FinishedAt:  time.Now(),
				GeneratedAt: generatedAt,
			})).To(Succeed())

			tok := mustIssueToken(authnSvc, owner)

			code, body := doServerReq(h, http.MethodGet, "/v1/jobs/"+jobID, tok, nil)
			Expect(code).To(Equal(http.StatusOK), "body=%s", body)
			var status coachapi.JobStatusResponse
			Expect(json.Unmarshal(body, &status)).To(Succeed())
			Expect(status.Status).To(Equal(coachapi.JobStatusCompleted))
			Expect(status.ReportURL).To(Equal("/v1/jobs/" + jobID + "/report"))

			code, body = doServerReq(h, http.MethodGet, "/v1/jobs/"+jobID+"/report", tok, nil)
			Expect(code).To(Equal(http.StatusOK), "body=%s", body)
			var report coachapi.Report
			Expect(json.Unmarshal(body, &report)).To(Succeed())
			Expect(report.ReportVersion).To(Equal(coachapi.ReportVersion1))
			Expect(report.JobID).To(Equal(jobID))
			Expect(report.Kind).To(Equal(coachapi.JobKindRepoBaselineScan))
			Expect(report.CommitSHA).To(Equal("abc123def4567890abc123def4567890abc123de"))
			Expect(report.Findings).NotTo(BeNil())
			Expect(report.Diagnostics).NotTo(BeNil())
		})
	})

	When("TaskQueue.Enqueue fails after the job row was durably persisted", func() {
		It("returns a 5xx (not 202) and leaves the job status queued, not failed", func() {
			store := newSpyJobStore()
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{enqueueErr: errors.New("queue: broker unavailable")}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("enqfail"))
			h := authnSvc.Middleware(srv.Handler())
			tok := mustIssueToken(authnSvc, principalAlice())

			code, body := doServerReq(h, http.MethodPost, "/v1/jobs", tok, validRepoBaselineScanBody())
			Expect(code).To(BeNumerically(">=", 500), "body=%s", body)
			Expect(code).To(BeNumerically("<", 600), "body=%s", body)
			env := decodeServerEnvelope(body)
			Expect(env.Error.Code).To(Equal(coachapi.ErrorCodeInternalError))

			Expect(store.createJobCalls()).To(Equal(1), "the job row must have been persisted before enqueue was attempted")

			jobID := "enqfail-1"
			code, body = doServerReq(h, http.MethodGet, "/v1/jobs/"+jobID, tok, nil)
			Expect(code).To(Equal(http.StatusOK), "body=%s", body)
			var status coachapi.JobStatusResponse
			Expect(json.Unmarshal(body, &status)).To(Succeed())
			Expect(status.Status).To(Equal(coachapi.JobStatusQueued), "enqueue failure must not mark the job failed")
		})
	})

	Describe("denylist-store error vs denylisted jti on POST /v1/jobs", func() {
		It("fails closed with 503 internal_error when the denylist store errors", func() {
			dl := &serverErrDenylist{err: errors.New("denylist unavailable")}
			svc := newAuthnServiceForServer(authn.Options{Denylist: dl})
			store := coachapi.NewMemoryStore()
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("dlerr"))
			h := svc.Middleware(srv.Handler())
			tok := mustIssueToken(svc, principalAlice())

			code, body := doServerReq(h, http.MethodPost, "/v1/jobs", tok, validRepoBaselineScanBody())
			expectEnvelope(code, body, http.StatusServiceUnavailable, coachapi.ErrorCodeInternalError)
		})

		It("returns 401 unauthenticated, distinct from the 503 store-error case, for a revoked jti", func() {
			svc := newAuthnServiceForServer(authn.Options{})
			store := coachapi.NewMemoryStore()
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("dlrevoked"))
			h := svc.Middleware(srv.Handler())
			tok := mustIssueToken(svc, principalAlice())
			Expect(svc.Revoke(context.Background(), tok)).To(Succeed())

			code, body := doServerReq(h, http.MethodPost, "/v1/jobs", tok, validRepoBaselineScanBody())
			expectEnvelope(code, body, http.StatusUnauthorized, coachapi.ErrorCodeUnauthenticated)
		})
	})
})

// serverErrDenylist always returns a store error from IsRevoked (fail-closed path).
type serverErrDenylist struct {
	err error
	mu  sync.Mutex
}

func (e *serverErrDenylist) IsRevoked(context.Context, string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return false, e.err
}

func (e *serverErrDenylist) Revoke(context.Context, string, time.Time) error {
	return nil
}
