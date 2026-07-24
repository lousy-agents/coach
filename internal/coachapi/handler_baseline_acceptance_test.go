package coachapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

// uuidShape matches the UUID PRIMARY KEY shape Postgres job_findings/job_diagnostics use.
var uuidShape = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// captureWriter records fenced writes and enforces the Postgres invariants the
// real store would reject: non-empty UUID primary keys and UNIQUE NULLS NOT
// DISTINCT (job_id, attempt, source, rubric_id, payload_hash).
type captureWriter struct {
	lease       coachapi.ClaimLease
	findings    []coachapi.JobFinding
	diagnostics []coachapi.JobDiagnostic
}

func (w *captureWriter) Lease() coachapi.ClaimLease { return w.lease }

func (w *captureWriter) InsertFindings(_ context.Context, findings []coachapi.JobFinding) error {
	seenID := map[string]struct{}{}
	seenUniq := map[string]struct{}{}
	for _, existing := range w.findings {
		if existing.ID != "" {
			seenID[existing.ID] = struct{}{}
		}
		seenUniq[findingUniqKey(existing)] = struct{}{}
	}
	for _, f := range findings {
		if f.ID == "" {
			return errors.New("coachapi: job_findings.id must be a non-empty UUID (postgres UUID PRIMARY KEY)")
		}
		if !uuidShape.MatchString(f.ID) {
			return fmt.Errorf("coachapi: job_findings.id %q is not UUID-shaped", f.ID)
		}
		if _, dup := seenID[f.ID]; dup {
			return fmt.Errorf("coachapi: duplicate job_findings.id %q", f.ID)
		}
		seenID[f.ID] = struct{}{}
		if f.PayloadHash == "" {
			return errors.New("coachapi: job_findings.payload_hash must be non-empty")
		}
		key := findingUniqKey(f)
		if _, dup := seenUniq[key]; dup {
			return fmt.Errorf("coachapi: duplicate job_findings unique key %s (UNIQUE NULLS NOT DISTINCT)", key)
		}
		seenUniq[key] = struct{}{}
	}
	w.findings = append(w.findings, findings...)
	return nil
}

func (w *captureWriter) InsertDiagnostics(_ context.Context, diagnostics []coachapi.JobDiagnostic) error {
	seenID := map[string]struct{}{}
	for _, existing := range w.diagnostics {
		if existing.ID != "" {
			seenID[existing.ID] = struct{}{}
		}
	}
	for _, d := range diagnostics {
		if d.ID == "" {
			return errors.New("coachapi: job_diagnostics.id must be a non-empty UUID (postgres UUID PRIMARY KEY)")
		}
		if !uuidShape.MatchString(d.ID) {
			return fmt.Errorf("coachapi: job_diagnostics.id %q is not UUID-shaped", d.ID)
		}
		if _, dup := seenID[d.ID]; dup {
			return fmt.Errorf("coachapi: duplicate job_diagnostics.id %q", d.ID)
		}
		seenID[d.ID] = struct{}{}
	}
	w.diagnostics = append(w.diagnostics, diagnostics...)
	return nil
}

func findingUniqKey(f coachapi.JobFinding) string {
	rubric := ""
	if f.RubricID != nil {
		rubric = *f.RubricID
	}
	// Mirrors UNIQUE NULLS NOT DISTINCT (job_id, attempt, source, rubric_id, payload_hash).
	// JobID/Attempt are stamped by leaseWriter in production; captureWriter keys on source+rubric+hash.
	return string(f.Source) + "\x00" + rubric + "\x00" + f.PayloadHash
}

var _ coachapi.BaselineJobWriter = (*captureWriter)(nil)

// memoryFencedWriter is a leaseWriter-equivalent over MemoryStore so acceptance
// exercises the real fenced InsertFindings/InsertDiagnostics path.
type memoryFencedWriter struct {
	store *coachapi.MemoryStore
	lease coachapi.ClaimLease
}

func (w *memoryFencedWriter) Lease() coachapi.ClaimLease { return w.lease }

func (w *memoryFencedWriter) InsertFindings(ctx context.Context, findings []coachapi.JobFinding) error {
	// Reuse captureWriter validation so empty IDs / hash collisions fail closed
	// the way Postgres would (MemoryStore itself does not enforce those).
	cap := &captureWriter{lease: w.lease}
	if err := cap.InsertFindings(ctx, findings); err != nil {
		return err
	}
	stamped := append([]coachapi.JobFinding(nil), findings...)
	for i := range stamped {
		stamped[i].JobID = w.lease.JobID
		stamped[i].Attempt = w.lease.Attempt
	}
	return w.store.InsertFindings(ctx, w.lease.JobID, w.lease.WorkerID, w.lease.Attempt, stamped)
}

func (w *memoryFencedWriter) InsertDiagnostics(ctx context.Context, diagnostics []coachapi.JobDiagnostic) error {
	cap := &captureWriter{lease: w.lease}
	if err := cap.InsertDiagnostics(ctx, diagnostics); err != nil {
		return err
	}
	stamped := append([]coachapi.JobDiagnostic(nil), diagnostics...)
	for i := range stamped {
		stamped[i].JobID = w.lease.JobID
		stamped[i].Attempt = w.lease.Attempt
	}
	return w.store.InsertDiagnostics(ctx, w.lease.JobID, w.lease.WorkerID, w.lease.Attempt, stamped)
}

var _ coachapi.BaselineJobWriter = (*memoryFencedWriter)(nil)

func newMemoryFencedWriter(job coachapi.Job) (*coachapi.MemoryStore, *memoryFencedWriter) {
	GinkgoHelper()
	store := coachapi.NewMemoryStore()
	queued := job
	queued.Status = coachapi.JobStatusQueued
	Expect(store.CreateJob(context.Background(), queued)).To(Succeed())
	lease, err := store.ClaimJob(context.Background(), job.ID, "baseline-test-worker", time.Now().UTC(), time.Minute)
	Expect(err).NotTo(HaveOccurred())
	return store, &memoryFencedWriter{store: store, lease: lease}
}

// fakeTreeSource is a test double for GitHub-backed tree fetch failures and budgets.
type fakeTreeSource struct {
	listErr   error
	readErr   error
	entries   []coachapi.BaselineFileEntry
	contents  map[string][]byte
	listCalls int
	readCalls int
}

func (f *fakeTreeSource) ListFiles(_ context.Context, _, _, _ string, _ coachapi.BaselineListOptions) ([]coachapi.BaselineFileEntry, error) {
	f.listCalls++
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]coachapi.BaselineFileEntry, len(f.entries))
	copy(out, f.entries)
	return out, nil
}

func (f *fakeTreeSource) ReadFile(_ context.Context, _, _, _, path string) ([]byte, string, error) {
	f.readCalls++
	if f.readErr != nil {
		return nil, "", f.readErr
	}
	if f.contents == nil {
		return nil, "", githubingest.ErrNotFound
	}
	b, ok := f.contents[path]
	if !ok {
		return nil, "", githubingest.ErrNotFound
	}
	return append([]byte(nil), b...), "blob-sha", nil
}

func baselineFixtureRoot() string {
	GinkgoHelper()
	_, thisFile, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue())
	root := filepath.Join(filepath.Dir(thisFile), "testdata", "baseline_fixture")
	_, err := os.Stat(root)
	Expect(err).NotTo(HaveOccurred(), "baseline fixture root must exist at %s", root)
	return root
}

func baselineJob(params coachapi.RepoBaselineScanParams) coachapi.Job {
	GinkgoHelper()
	raw, err := json.Marshal(params)
	Expect(err).NotTo(HaveOccurred())
	return coachapi.Job{
		ID:                "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		Kind:              coachapi.JobKindRepoBaselineScan,
		Params:            raw,
		Status:            coachapi.JobStatusRunning,
		Attempt:           1,
		CreatedAt:         time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC),
		CreatedByProvider: "github",
		CreatedBySubject:  "1",
		CreatedByLogin:    "octocat",
	}
}

func newCaptureWriter() *captureWriter {
	return &captureWriter{
		lease: coachapi.ClaimLease{
			JobID:    "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
			WorkerID: "baseline-test-worker",
			Attempt:  1,
		},
	}
}

func handlerSourcedNames(calls []agentloop.RecordedCall) []string {
	var names []string
	for _, c := range calls {
		if c.Source == agentloop.CallSourceHandler {
			names = append(names, c.Name)
		}
	}
	return names
}

var _ = Describe("repo_baseline_scan job handler", func() {
	When("worker is configured with a local smoke fixture path", func() {
		It("completes a baseline via agentloop against the fixture and records deterministic findings", func() {
			var observed *agentloop.Loop
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "smoke-owner",
				SmokeRepoName:    "smoke-repo",
				Gateway:          modelgateway.NewStubGateway(),
				ObserveLoop: func(loop *agentloop.Loop) {
					observed = loop
				},
			})

			job := baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "smoke-owner",
				RepoName:  "smoke-repo",
				Ref:       "main",
			})
			// MemoryStore + lease fencing + Postgres-shaped ID/unique-hash checks.
			_, w := newMemoryFencedWriter(job)
			completion, err := h(context.Background(), job, w)
			Expect(err).NotTo(HaveOccurred(),
				"InsertFindings must mint UUID ids and unique payload_hash values (postgres PK/UNIQUE)")
			Expect(completion).NotTo(BeNil())
			Expect(completion.CommitSHA).To(Equal("local-fixture"))
			Expect(completion.Versions.Analyzer).NotTo(BeEmpty())
			Expect(completion.Versions.Rubrics).To(HaveKey(rubrics.IDHiddenMutationContextualization))
			Expect(completion.Versions.Rubrics).To(HaveKey(rubrics.IDChangeCohesion))

			// Second pass through captureWriter to inspect minted IDs/hashes directly.
			cap := newCaptureWriter()
			_, err = h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "smoke-owner",
				RepoName:  "smoke-repo",
				Ref:       "main",
			}), cap)
			Expect(err).NotTo(HaveOccurred())

			var det, agent int
			uniqKeys := map[string]struct{}{}
			for _, f := range cap.findings {
				Expect(f.ID).NotTo(BeEmpty(), "every finding requires a minted UUID id")
				Expect(f.ID).To(MatchRegexp(uuidShape.String()), "finding id must be UUID-shaped for postgres")
				Expect(f.PayloadHash).NotTo(BeEmpty(), "every finding requires a stable payload_hash")
				Expect(f.Payload).NotTo(BeEmpty())
				key := findingUniqKey(f)
				_, dup := uniqKeys[key]
				Expect(dup).To(BeFalse(), "payload_hash must be unique per (source, rubric_id): %s", key)
				uniqKeys[key] = struct{}{}
				switch f.Source {
				case coachapi.FindingSourceDeterministic:
					det++
				case coachapi.FindingSourceAgent:
					agent++
					Expect(f.RubricID).NotTo(BeNil())
					Expect(f.RubricVersion).NotTo(BeNil())
					Expect(f.ModelIdentity).NotTo(BeNil())
				}
			}
			for _, d := range cap.diagnostics {
				Expect(d.ID).NotTo(BeEmpty(), "every diagnostic requires a minted UUID id")
				Expect(d.ID).To(MatchRegexp(uuidShape.String()), "diagnostic id must be UUID-shaped for postgres")
			}
			Expect(det).To(BeNumerically(">=", 1),
				"fixture widget/*.go must produce at least one deterministic codesignal signal")
			Expect(agent).To(BeNumerically(">=", 1),
				"stub gateway should yield at least one source=agent judgment finding")

			Expect(observed).NotTo(BeNil(), "handler must construct an agentloop for the analysis path")
		})

		It("persists distinct agent payload_hash values for multiple hidden_mutation signals", func() {
			// Fixture has ≥2 hidden_input_mutation signals (widget/update.go + widget/reset.go).
			// Stub judgments are identical across signals, so agent payload_hash must include a
			// per-signal discriminator or UNIQUE (job_id, attempt, source, rubric_id, payload_hash) fails.
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "smoke-owner",
				SmokeRepoName:    "smoke-repo",
				Gateway:          modelgateway.NewStubGateway(),
			})

			job := baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "smoke-owner",
				RepoName:  "smoke-repo",
			})
			_, w := newMemoryFencedWriter(job)
			completion, err := h(context.Background(), job, w)
			Expect(err).NotTo(HaveOccurred(),
				"multi hidden-mutation agent findings must not collide on payload_hash")
			Expect(completion).NotTo(BeNil())

			// Capture a clean write for hash inspection (same handler path).
			cap := newCaptureWriter()
			_, err = h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "smoke-owner",
				RepoName:  "smoke-repo",
			}), cap)
			Expect(err).NotTo(HaveOccurred())

			var hiddenAgentHashes []string
			var detHidden int
			for _, f := range cap.findings {
				Expect(f.ID).To(MatchRegexp(uuidShape.String()))
				switch f.Source {
				case coachapi.FindingSourceDeterministic:
					if bytes.Contains(f.Payload, []byte(`"hidden_input_mutation"`)) ||
						bytes.Contains(f.Payload, []byte(`state.hidden_input_mutation`)) {
						detHidden++
					}
				case coachapi.FindingSourceAgent:
					if f.RubricID != nil && *f.RubricID == rubrics.IDHiddenMutationContextualization {
						hiddenAgentHashes = append(hiddenAgentHashes, f.PayloadHash)
					}
				}
			}
			Expect(detHidden).To(BeNumerically(">=", 2),
				"fixture must yield ≥2 deterministic hidden_input_mutation signals")
			Expect(hiddenAgentHashes).To(HaveLen(detHidden),
				"one agent judgment finding per hidden-mutation deterministic signal")
			Expect(hiddenAgentHashes[0]).NotTo(Equal(hiddenAgentHashes[1]),
				"agent payload_hash values for distinct signals must differ")
			uniq := map[string]struct{}{}
			for _, hsh := range hiddenAgentHashes {
				uniq[hsh] = struct{}{}
			}
			Expect(uniq).To(HaveLen(len(hiddenAgentHashes)),
				"all hidden_mutation agent payload_hash values must be unique")
		})

		It("records handler-sourced semantics_analyze and codesignal_report calls on the loop", func() {
			var observed *agentloop.Loop
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "smoke-owner",
				SmokeRepoName:    "smoke-repo",
				Gateway:          modelgateway.NewStubGateway(),
				ObserveLoop: func(loop *agentloop.Loop) {
					observed = loop
				},
			})

			w := newCaptureWriter()
			_, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "smoke-owner",
				RepoName:  "smoke-repo",
			}), w)
			Expect(err).NotTo(HaveOccurred())
			Expect(observed).NotTo(BeNil())

			names := handlerSourcedNames(observed.Calls())
			Expect(names).To(ContainElement(agentloop.ToolSemanticsAnalyze),
				"analysis must go through agentloop.Call(handler, semantics_analyze); no direct pkg/semantics bypass")
			Expect(names).To(ContainElement(agentloop.ToolCodeSignalReport),
				"analysis must go through agentloop.Call(handler, codesignal_report); no direct pkg/codesignal bypass")
			// Seed rubrics also handler-driven.
			Expect(names).To(ContainElement(rubrics.IDChangeCohesion))
		})
	})

	When("the repository exceeds the configured size budget", func() {
		It("fails the job with an actionable too-large error", func() {
			// Two fixture .go files; budget of 1 file must fail before analysis.
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "smoke-owner",
				SmokeRepoName:    "smoke-repo",
				MaxFiles:         1,
				Gateway:          modelgateway.NewStubGateway(),
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "smoke-owner",
				RepoName:  "smoke-repo",
			}), w)
			Expect(completion).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, githubingest.ErrTooLarge)).To(BeTrue(),
				"oversized path must wrap githubingest.ErrTooLarge (or coach equivalent wrapping it); got %v", err)
			Expect(err.Error()).To(Or(
				ContainSubstring("budget"),
				ContainSubstring("too large"),
				ContainSubstring("exceeds"),
				ContainSubstring("MaxFiles"),
				ContainSubstring("max files"),
			))
		})
	})

	When("GitHub fetch fails with not-found/auth", func() {
		It("fails with a sentinel-mapped actionable error", func() {
			src := &fakeTreeSource{listErr: githubingest.ErrNotFound}
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				TreeSource: src,
				Gateway:    modelgateway.NewStubGateway(),
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "acme",
				RepoName:  "missing",
				Ref:       "main",
			}), w)
			Expect(completion).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, githubingest.ErrNotFound)).To(BeTrue(),
				"not-found fetch must remain errors.Is-compatible with githubingest.ErrNotFound; got %v", err)
			Expect(err.Error()).NotTo(BeEmpty())
			Expect(src.listCalls).To(BeNumerically(">=", 1))

			srcAuth := &fakeTreeSource{listErr: githubingest.ErrAuth}
			hAuth := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				TreeSource: srcAuth,
				Gateway:    modelgateway.NewStubGateway(),
			})
			_, err = hAuth(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "acme",
				RepoName:  "private",
			}), newCaptureWriter())
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, githubingest.ErrAuth)).To(BeTrue(),
				"auth fetch must remain errors.Is-compatible with githubingest.ErrAuth; got %v", err)
		})
	})

	When("the model gateway is unavailable for judgment", func() {
		It("still completes with deterministic findings and judgment diagnostics", func() {
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "smoke-owner",
				SmokeRepoName:    "smoke-repo",
				Gateway: modelgateway.NewStubGateway(modelgateway.StubOptions{
					JudgeErr: modelgateway.NewUnavailableError("gateway down", nil),
				}),
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "smoke-owner",
				RepoName:  "smoke-repo",
			}), w)
			Expect(err).NotTo(HaveOccurred(), "judgment degrade must not fail the job (Story 5)")
			Expect(completion).NotTo(BeNil())

			allFindings := append([]coachapi.JobFinding{}, w.findings...)
			allFindings = append(allFindings, completion.Findings...)
			var det int
			for _, f := range allFindings {
				if f.Source == coachapi.FindingSourceDeterministic {
					det++
				}
				Expect(f.Source).NotTo(Equal(coachapi.FindingSourceAgent),
					"unavailable gateway must not produce source=agent findings")
			}
			Expect(det).To(BeNumerically(">=", 1))

			allDiags := append([]coachapi.JobDiagnostic{}, w.diagnostics...)
			allDiags = append(allDiags, completion.Diagnostics...)
			Expect(allDiags).NotTo(BeEmpty(), "judgment degrade must record JobDiagnostic entries")
			var sawRubricScope bool
			for _, d := range allDiags {
				if len(d.Scope) > 0 && (d.Scope == "rubric:"+rubrics.IDHiddenMutationContextualization ||
					d.Scope == "rubric:"+rubrics.IDChangeCohesion ||
					len(d.Scope) >= 7 && d.Scope[:7] == "rubric:") {
					sawRubricScope = true
				}
			}
			Expect(sawRubricScope).To(BeTrue(), "diagnostics should scope to rubric:* from degrade envelopes")
		})
	})

	When("client-supplied clone URLs appear in job params", func() {
		It("rejects git_url at the public params decode boundary (API contract; submit matrix is Task 2)", func() {
			// Cross-check of server DisallowUnknownFields contract without re-testing RepoAuthorizer.
			raw := []byte(`{"repo_owner":"acme","repo_name":"widgets","git_url":"https://evil.example/x.git"}`)
			dec := json.NewDecoder(bytes.NewReader(raw))
			dec.DisallowUnknownFields()
			var params coachapi.RepoBaselineScanParams
			err := dec.Decode(&params)
			Expect(err).To(HaveOccurred(), "git_url must be rejected at the params schema boundary")

			// Belt-and-suspenders: handler must also fail permanently if forbidden keys sneak into stored params.
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "acme",
				SmokeRepoName:    "widgets",
				Gateway:          modelgateway.NewStubGateway(),
			})
			job := baselineJob(coachapi.RepoBaselineScanParams{RepoOwner: "acme", RepoName: "widgets"})
			job.Params = raw
			_, err = h(context.Background(), job, newCaptureWriter())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Or(
				ContainSubstring("git_url"),
				ContainSubstring("clone_url"),
				ContainSubstring("not allowed"),
				ContainSubstring("invalid"),
			))
		})
	})
})
