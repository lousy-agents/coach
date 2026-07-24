package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/worker"
	"github.com/lousy-agents/coach/internal/fakegithub"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

func workerRSAKey() []byte {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return pem.EncodeToMemory(block)
}

var _ = Describe("coach-worker baseline handler wiring", func() {
	When("GitHub AppID and private key are configured without a static InstallationID", func() {
		It("builds a CredentialResolver-backed tree source that resolves installation per repo", func() {
			fx := fakegithub.NewFixture("worker-resolver-fixture")
			const objectSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			fx.Installation.Installations[99] = fakegithub.InstallationEntry{
				Token: "worker-install-token", Scenario: fakegithub.ScenarioOK,
			}
			fx.Installation.RepoMappings["acme/widgets"] = fakegithub.RepoInstallationEntry{
				InstallationID: 99, Scenario: fakegithub.ScenarioOK,
			}
			fx.Repos.Repos["acme/widgets"] = fakegithub.RepoMetaEntry{
				DefaultBranch: "main", Scenario: fakegithub.ScenarioOK,
			}
			fx.Repos.Commits["acme/widgets/main"] = fakegithub.CommitEntry{
				SHA: objectSHA, Scenario: fakegithub.ScenarioOK,
			}
			// Minimal tree for a successful baseline (one tiny go file).
			// Root dir listing + file content + parent dir for symlink check (root itself).
			body := []byte("package main\n\nfunc main() {}\n")
			fx.Contents.Dirs["acme/widgets/"+objectSHA] = []fakegithub.DirEntry{
				{Name: "main.go", Type: "file", SHA: "blob1", Size: len(body)},
			}
			fx.Contents.Files["acme/widgets/"+objectSHA+"/main.go"] = fakegithub.FileEntry{
				Content: body, SHA: "blob1", Scenario: fakegithub.ScenarioOK,
			}
			server := fakegithub.NewServer(&fx)
			DeferCleanup(server.Close)

			cfg := Config{
				GitHubAppID:      12345,
				GitHubPrivateKey: workerRSAKey(),
				GitHubBaseURL:    server.URL(),
				// InstallationID intentionally zero — must resolve per repo.
			}
			h, err := buildJobHandler(cfg)
			Expect(err).NotTo(HaveOccurred())

			job := coachapi.Job{
				ID:     "cccccccc-cccc-cccc-cccc-cccccccccccc",
				Kind:   coachapi.JobKindRepoBaselineScan,
				Params: []byte(`{"repo_owner":"acme","repo_name":"widgets","ref":"main"}`),
				Status: coachapi.JobStatusRunning,
			}
			store := coachapi.NewMemoryStore()
			Expect(store.CreateJob(context.Background(), coachapi.Job{
				ID: job.ID, Kind: job.Kind, Params: job.Params,
				Status: coachapi.JobStatusQueued, Attempt: 0,
				CreatedByProvider: "github", CreatedBySubject: "1", CreatedByLogin: "octocat",
			})).To(Succeed())
			lease, err := store.ClaimJob(context.Background(), job.ID, "w1", storeNow(), storeStale())
			Expect(err).NotTo(HaveOccurred())

			// Minimal JobWriter adapter over MemoryStore fencing.
			w := &storeJobWriter{store: store, lease: lease}
			completion, err := h(context.Background(), job, w)
			Expect(err).NotTo(HaveOccurred())
			Expect(completion).NotTo(BeNil())
			Expect(completion.CommitSHA).To(Equal(objectSHA),
				"worker path must resolve commit object SHA via CredentialResolver-backed reader")
			Expect(completion.CommitSHA).NotTo(Equal("main"))
		})
	})

	When("baseline fetch fails with a transient non-sentinel error", func() {
		It("wraps the error as worker.Retryable", func() {
			err := classifyBaselineHandlerError(fmt.Errorf("coachapi: baseline fetch failed: %w", errors.New("connection refused")))
			Expect(worker.IsRetryable(err)).To(BeTrue(), "transient fetch must be Retryable; got %v", err)
		})

		It("keeps ErrNotFound permanent", func() {
			err := classifyBaselineHandlerError(fmt.Errorf("coachapi: baseline fetch failed: %w", githubingest.ErrNotFound))
			Expect(worker.IsRetryable(err)).To(BeFalse())
			Expect(errors.Is(err, githubingest.ErrNotFound)).To(BeTrue())
		})

		It("keeps ErrAuth and ErrTooLarge permanent", func() {
			for _, sent := range []error{githubingest.ErrAuth, githubingest.ErrTooLarge} {
				err := classifyBaselineHandlerError(fmt.Errorf("coachapi: baseline fetch failed: %w", sent))
				Expect(worker.IsRetryable(err)).To(BeFalse(), "sentinel %v", sent)
			}
		})

		It("keeps bad params permanent", func() {
			err := classifyBaselineHandlerError(errors.New("coachapi: repo_owner and repo_name are required"))
			Expect(worker.IsRetryable(err)).To(BeFalse())
		})
	})

	When("only a smoke fixture is configured (credential-free)", func() {
		It("builds a handler that completes without GitHub credentials", func() {
			root := GinkgoT().TempDir()
			Expect(os.WriteFile(filepath.Join(root, "a.go"), []byte("package a\n"), 0o644)).To(Succeed())

			h, err := buildJobHandler(Config{
				SmokeFixturePath: root,
				SmokeRepoOwner:   "smoke",
				SmokeRepoName:    "fixture",
			})
			Expect(err).NotTo(HaveOccurred())

			job := coachapi.Job{
				ID:     "dddddddd-dddd-dddd-dddd-dddddddddddd",
				Kind:   coachapi.JobKindRepoBaselineScan,
				Params: []byte(`{"repo_owner":"smoke","repo_name":"fixture"}`),
				Status: coachapi.JobStatusRunning,
			}
			store := coachapi.NewMemoryStore()
			Expect(store.CreateJob(context.Background(), coachapi.Job{
				ID: job.ID, Kind: job.Kind, Params: job.Params,
				Status: coachapi.JobStatusQueued, Attempt: 0,
				CreatedByProvider: "github", CreatedBySubject: "1", CreatedByLogin: "octocat",
			})).To(Succeed())
			lease, err := store.ClaimJob(context.Background(), job.ID, "w1", storeNow(), storeStale())
			Expect(err).NotTo(HaveOccurred())

			completion, err := h(context.Background(), job, &storeJobWriter{store: store, lease: lease})
			Expect(err).NotTo(HaveOccurred())
			Expect(completion.CommitSHA).To(Equal("local-fixture"))
		})
	})

	When("MODEL_GATEWAY_BASE_URL is set but the OpenAI-compat client cannot be constructed", func() {
		It("degrades to ErrUnavailable rather than the success stub's canned judgments", func() {
			// ConfigFromEnv accepts any non-empty URL; NewOpenAICompatClient rejects this.
			Expect(os.Setenv("MODEL_GATEWAY_BASE_URL", "://not-a-valid-url")).To(Succeed())
			DeferCleanup(func() { _ = os.Unsetenv("MODEL_GATEWAY_BASE_URL") })

			gw := buildModelGateway()
			_, err := gw.Judge(context.Background(), modelgateway.JudgmentRequest{
				RubricID:      "change_cohesion",
				RubricVersion: "1",
				OutputSchema:  []byte(`{"type":"object"}`),
				Messages:      []modelgateway.Message{{Role: "user", Content: "x"}},
			})
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, modelgateway.ErrUnavailable)).To(BeTrue(),
				"misconfigured gateway must degrade as unavailable, not emit canned agent judgments; got %v", err)
		})
	})
})

// storeJobWriter is a thin JobWriter over MemoryStore for composition tests.
type storeJobWriter struct {
	store *coachapi.MemoryStore
	lease coachapi.ClaimLease
}

func (w *storeJobWriter) Lease() coachapi.ClaimLease { return w.lease }

func (w *storeJobWriter) InsertFindings(ctx context.Context, findings []coachapi.JobFinding) error {
	return w.store.InsertFindings(ctx, w.lease.JobID, w.lease.WorkerID, w.lease.Attempt, findings)
}

func (w *storeJobWriter) InsertDiagnostics(ctx context.Context, diagnostics []coachapi.JobDiagnostic) error {
	return w.store.InsertDiagnostics(ctx, w.lease.JobID, w.lease.WorkerID, w.lease.Attempt, diagnostics)
}

func storeNow() time.Time { return time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC) }

func storeStale() time.Duration { return time.Minute }
