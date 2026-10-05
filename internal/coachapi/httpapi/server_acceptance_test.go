package httpapi_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/authn"
	"github.com/lousy-agents/coach/internal/authz"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/httpapi"
	"github.com/lousy-agents/coach/internal/coachapi/queue"
	"github.com/lousy-agents/coach/internal/coachapi/store/memory"
	"github.com/lousy-agents/coach/internal/fakegithub"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

var _ = Describe("httpapi.Server HTTP surface (POST /v1/jobs, GET /v1/jobs/{id}, GET /v1/jobs/{id}/report)", func() {
	var (
		authnSvc *authn.Service
	)

	BeforeEach(func() {
		authnSvc = newAuthnServiceForServer(authn.Options{})
	})

	When("an authenticated principal submits a valid repo_baseline_scan job and a nil-erroring authorizer allows it", func() {
		It("returns 202 with an id, persists created_by_* from the principal, GET status shows queued, and the queue records a versioned payload with the job id", func() {
			store := newSpyJobStore()
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC)), sequentialJobIDs("happy"))
			h := authnSvc.Middleware(srv.Handler())

			owner := principalAlice()
			tok := mustIssueToken(authnSvc, owner)
			code, body := doServerReq(h, http.MethodPost, "/v1/jobs", tok, validRepoBaselineScanBody())
			Expect(code).To(Equal(http.StatusAccepted), "body=%s", body)
			var created coachapi.CreateJobResponse
			Expect(json.Unmarshal(body, &created)).To(Succeed())
			Expect(created.ID).NotTo(BeEmpty())

			job, err := store.GetJob(context.Background(), created.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(job.CreatedByProvider).To(Equal(owner.Provider), "created_by_provider must equal principal.provider")
			Expect(job.CreatedBySubject).To(Equal(owner.Subject), "created_by_subject must equal principal.subject")
			Expect(job.CreatedByLogin).To(Equal(owner.Login), "created_by_login must equal principal.login at submit (audit/display)")

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

		It("rejects a clone_url key present in params (client-supplied clone URL equivalent)", func() {
			body := []byte(`{"kind":"repo_baseline_scan","params":{"repo_owner":"acme","repo_name":"widgets","clone_url":"https://evil.example/x.git"}}`)
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
			store := memory.NewStore()
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
		It("returns 403 repo_not_authorized with Story 3's actionable public-repo message and persists nothing", func() {
			store := newSpyJobStore()
			az := &stubRepoAuthorizer{err: fmt.Errorf("stub: no role: %w", authz.ErrNotAuthorized)}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("notauthz"))
			h := authnSvc.Middleware(srv.Handler())
			tok := mustIssueToken(authnSvc, principalAlice())

			code, body := doServerReq(h, http.MethodPost, "/v1/jobs", tok, validRepoBaselineScanBody())
			env := expectEnvelope(code, body, http.StatusForbidden, coachapi.ErrorCodeRepoNotAuthorized)
			// Story 3: pilots must not report public-repo denial as a bug.
			Expect(env.Error.Message).To(ContainSubstring("no role"))
			Expect(env.Error.Message).To(ContainSubstring("public repositories"))
			Expect(env.Error.Message).To(ContainSubstring("deliberately denied"))
			Expect(store.createJobCalls()).To(Equal(0))
		})
	})

	When("submit-time authz uses Story 3's config-gated BypassAuthorizer at the HTTP boundary", func() {
		It("returns 202 for the exact bypass pair without consulting the denying live authorizer", func() {
			store := newSpyJobStore()
			inner := &stubRepoAuthorizer{err: authz.ErrNotAuthorized}
			az := authz.NewBypassAuthorizer(inner, "acme", "widgets")
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("bypass-on"))
			h := authnSvc.Middleware(srv.Handler())
			tok := mustIssueToken(authnSvc, principalAlice())

			code, body := doServerReq(h, http.MethodPost, "/v1/jobs", tok, validRepoBaselineScanBody())
			Expect(code).To(Equal(http.StatusAccepted), "body=%s", body)
			Expect(inner.callCount()).To(Equal(0), "exact bypass pair must skip the live authorizer")
			Expect(store.createJobCalls()).To(Equal(1))
		})

		It("returns 403 repo_not_authorized for a non-matching pair and still runs the live authorizer", func() {
			store := newSpyJobStore()
			inner := &stubRepoAuthorizer{err: authz.ErrNotAuthorized}
			az := authz.NewBypassAuthorizer(inner, "acme", "widgets")
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("bypass-miss"))
			h := authnSvc.Middleware(srv.Handler())
			tok := mustIssueToken(authnSvc, principalAlice())

			body := []byte(`{"kind":"repo_baseline_scan","params":{"repo_owner":"acme","repo_name":"other-repo"}}`)
			code, respBody := doServerReq(h, http.MethodPost, "/v1/jobs", tok, body)
			expectEnvelope(code, respBody, http.StatusForbidden, coachapi.ErrorCodeRepoNotAuthorized)
			Expect(inner.callCount()).To(Equal(1), "non-matching pairs must run the full live-check via inner")
			Expect(store.createJobCalls()).To(Equal(0))
		})

		It("returns 403 when bypass is unconfigured and the live authorizer denies (default-off)", func() {
			store := newSpyJobStore()
			// No BypassAuthorizer wrapper — production-like wiring when the
			// smoke fixture pair is not configured.
			az := &stubRepoAuthorizer{err: authz.ErrNotAuthorized}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("bypass-off"))
			h := authnSvc.Middleware(srv.Handler())
			tok := mustIssueToken(authnSvc, principalAlice())

			code, body := doServerReq(h, http.MethodPost, "/v1/jobs", tok, validRepoBaselineScanBody())
			expectEnvelope(code, body, http.StatusForbidden, coachapi.ErrorCodeRepoNotAuthorized)
			Expect(az.callCount()).To(Equal(1))
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
			store := memory.NewStore()
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
			store := memory.NewStore()
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
			store := memory.NewStore()
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

	When("a different principal reads a completed job's status and report", func() {
		It("returns 403 unauthorized on both routes (ADR-004 applies after completion too)", func() {
			store := memory.NewStore()
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("crossdone"))
			h := authnSvc.Middleware(srv.Handler())

			owner := principalAlice()
			jobID := "cross-completed-1"
			Expect(store.CreateJob(context.Background(), coachapi.Job{
				ID: jobID, Kind: coachapi.JobKindRepoBaselineScan,
				Params: json.RawMessage(`{"repo_owner":"acme","repo_name":"widgets"}`),
				Status: coachapi.JobStatusQueued, CreatedAt: time.Now(),
				CreatedByProvider: owner.Provider, CreatedBySubject: owner.Subject, CreatedByLogin: owner.Login,
			})).To(Succeed())
			Expect(store.RecordCompletion(context.Background(), jobID, coachapi.Completion{
				Attempt: 1, CommitSHA: "abc123def4567890abc123def4567890abc123de",
				Versions:   coachapi.ReportVersions{Analyzer: "codesignal@1"},
				FinishedAt: time.Now(), GeneratedAt: time.Date(2026, 7, 23, 13, 0, 0, 0, time.UTC),
			})).To(Succeed())

			bobTok := mustIssueToken(authnSvc, principalBob())
			code, body := doServerReq(h, http.MethodGet, "/v1/jobs/"+jobID, bobTok, nil)
			expectEnvelope(code, body, http.StatusForbidden, coachapi.ErrorCodeUnauthorized)

			code, body = doServerReq(h, http.MethodGet, "/v1/jobs/"+jobID+"/report", bobTok, nil)
			expectEnvelope(code, body, http.StatusForbidden, coachapi.ErrorCodeUnauthorized)
		})
	})

	When("the owning principal's GitHub login has been renamed since submit (same provider+subject)", func() {
		It("still authorizes GET status and report — ownership uses stable subject, not login (ADR-004)", func() {
			store := memory.NewStore()
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("rename"))
			h := authnSvc.Middleware(srv.Handler())
			// Job was created under the old login; token now carries the new login.
			jobID := "rename-job-1"
			Expect(store.CreateJob(context.Background(), coachapi.Job{
				ID: jobID, Kind: coachapi.JobKindRepoBaselineScan,
				Params: json.RawMessage(`{"repo_owner":"acme","repo_name":"widgets"}`),
				Status: coachapi.JobStatusQueued, CreatedAt: time.Now(),
				CreatedByProvider: "github", CreatedBySubject: "1001", CreatedByLogin: "alice",
			})).To(Succeed())
			Expect(store.RecordCompletion(context.Background(), jobID, coachapi.Completion{
				Attempt: 1, CommitSHA: "abc123def4567890abc123def4567890abc123de",
				Versions:   coachapi.ReportVersions{Analyzer: "codesignal@1"},
				FinishedAt: time.Now(), GeneratedAt: time.Date(2026, 7, 23, 13, 0, 0, 0, time.UTC),
			})).To(Succeed())

			renamed := coachapi.Principal{Provider: "github", Subject: "1001", Login: "alice-renamed"}
			tok := mustIssueToken(authnSvc, renamed)

			code, body := doServerReq(h, http.MethodGet, "/v1/jobs/"+jobID, tok, nil)
			Expect(code).To(Equal(http.StatusOK), "body=%s", body)

			code, body = doServerReq(h, http.MethodGet, "/v1/jobs/"+jobID+"/report", tok, nil)
			Expect(code).To(Equal(http.StatusOK), "body=%s", body)
		})
	})

	DescribeTable("409 job_not_completed for the owning principal, message mentions the actual status",
		func(seed func(store *memory.Store, jobID string, owner coachapi.Principal), wantStatusInMessage string) {
			store := memory.NewStore()
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
		Entry("queued", func(store *memory.Store, jobID string, owner coachapi.Principal) {
			Expect(store.CreateJob(context.Background(), coachapi.Job{
				ID: jobID, Kind: coachapi.JobKindRepoBaselineScan,
				Params: json.RawMessage(`{"repo_owner":"acme","repo_name":"widgets"}`),
				Status: coachapi.JobStatusQueued, CreatedAt: time.Now(),
				CreatedByProvider: owner.Provider, CreatedBySubject: owner.Subject, CreatedByLogin: owner.Login,
			})).To(Succeed())
		}, "queued"),
		Entry("running", func(store *memory.Store, jobID string, owner coachapi.Principal) {
			Expect(store.CreateJob(context.Background(), coachapi.Job{
				ID: jobID, Kind: coachapi.JobKindRepoBaselineScan,
				Params: json.RawMessage(`{"repo_owner":"acme","repo_name":"widgets"}`),
				Status: coachapi.JobStatusRunning, CreatedAt: time.Now(),
				CreatedByProvider: owner.Provider, CreatedBySubject: owner.Subject, CreatedByLogin: owner.Login,
			})).To(Succeed())
		}, "running"),
		Entry("failed", func(store *memory.Store, jobID string, owner coachapi.Principal) {
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
			store := memory.NewStore()
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
			store := memory.NewStore()
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
			store := memory.NewStore()
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

	Describe("JobStore dependency errors fail closed with 503 (not 404/500)", func() {
		It("returns 503 internal_error on POST /v1/jobs when CreateJob fails", func() {
			store := &errJobStore{createErr: errors.New("postgres: connection refused")}
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("store-create"))
			h := authnSvc.Middleware(srv.Handler())
			tok := mustIssueToken(authnSvc, principalAlice())

			code, body := doServerReq(h, http.MethodPost, "/v1/jobs", tok, validRepoBaselineScanBody())
			expectEnvelope(code, body, http.StatusServiceUnavailable, coachapi.ErrorCodeInternalError)
			Expect(q.enqueuedTasks()).To(BeEmpty(), "failed CreateJob must not enqueue")
		})

		It("returns 503 internal_error on GET /v1/jobs/{id} when GetJob fails with a non-not-found error", func() {
			store := &errJobStore{getErr: errors.New("postgres: connection refused")}
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("store-get"))
			h := authnSvc.Middleware(srv.Handler())
			tok := mustIssueToken(authnSvc, principalAlice())

			code, body := doServerReq(h, http.MethodGet, "/v1/jobs/any-id", tok, nil)
			expectEnvelope(code, body, http.StatusServiceUnavailable, coachapi.ErrorCodeInternalError)
		})

		It("returns 503 internal_error on GET /v1/jobs/{id}/report when GetJob fails with a non-not-found error", func() {
			store := &errJobStore{getErr: errors.New("postgres: connection refused")}
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("store-report-get"))
			h := authnSvc.Middleware(srv.Handler())
			tok := mustIssueToken(authnSvc, principalAlice())

			code, body := doServerReq(h, http.MethodGet, "/v1/jobs/any-id/report", tok, nil)
			expectEnvelope(code, body, http.StatusServiceUnavailable, coachapi.ErrorCodeInternalError)
		})

		It("returns 503 internal_error on GET report when GetJob succeeds but GetReport fails", func() {

			mem := memory.NewStore()
			owner := principalAlice()
			jobID := "store-report-fail-1"
			Expect(mem.CreateJob(context.Background(), coachapi.Job{
				ID: jobID, Kind: coachapi.JobKindRepoBaselineScan,
				Params: json.RawMessage(`{"repo_owner":"acme","repo_name":"widgets"}`),
				Status: coachapi.JobStatusQueued, CreatedAt: time.Now(),
				CreatedByProvider: owner.Provider, CreatedBySubject: owner.Subject, CreatedByLogin: owner.Login,
			})).To(Succeed())
			Expect(mem.RecordCompletion(context.Background(), jobID, coachapi.Completion{
				Attempt: 1, CommitSHA: "abc123def4567890abc123def4567890abc123de",
				Versions:   coachapi.ReportVersions{Analyzer: "codesignal@1"},
				FinishedAt: time.Now(), GeneratedAt: time.Date(2026, 7, 23, 13, 0, 0, 0, time.UTC),
			})).To(Succeed())

			store := &errJobStore{Store: mem, reportErr: errors.New("postgres: connection refused")}
			az := &stubRepoAuthorizer{}
			q := &stubTaskQueue{}
			srv := newTestServer(store, az, q, serverFixedNow(time.Now()), sequentialJobIDs("store-report"))
			h := authnSvc.Middleware(srv.Handler())
			tok := mustIssueToken(authnSvc, owner)

			code, body := doServerReq(h, http.MethodGet, "/v1/jobs/"+jobID+"/report", tok, nil)
			expectEnvelope(code, body, http.StatusServiceUnavailable, coachapi.ErrorCodeInternalError)
		})
	})
})

func newTestServer(store coachapi.JobStore, az authz.RepoAuthorizer, q queue.TaskQueue, now func() time.Time, newJobID func() string) *httpapi.Server {
	srv, err := httpapi.NewServer(httpapi.ServerConfig{
		Store:      store,
		Authorizer: az,
		Queue:      q,
		Now:        now,
		NewJobID:   newJobID,
	})
	Expect(err).NotTo(HaveOccurred())
	return srv
}
