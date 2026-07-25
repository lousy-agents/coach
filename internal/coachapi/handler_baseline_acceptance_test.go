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
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/fakegithub"
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
	listErr        error
	readErr        error
	resolveErr     error
	resolvedSHA    string
	entries        []coachapi.BaselineFileEntry
	contents       map[string][]byte
	listCalls      int
	readCalls      int
	resolveCalls   int
	lastListRef    string
	lastReadRef    string
	lastResolveRef string
}

func (f *fakeTreeSource) ResolveCommitSHA(_ context.Context, _, _, ref string) (string, error) {
	f.resolveCalls++
	f.lastResolveRef = ref
	if f.resolveErr != nil {
		return "", f.resolveErr
	}
	if f.resolvedSHA != "" {
		return f.resolvedSHA, nil
	}
	if ref == "" {
		return "HEAD", nil
	}
	return ref, nil
}

func (f *fakeTreeSource) ListFiles(_ context.Context, _, _, ref string, _ coachapi.BaselineListOptions) ([]coachapi.BaselineFileEntry, error) {
	f.listCalls++
	f.lastListRef = ref
	if f.listErr != nil {
		return nil, f.listErr
	}
	out := make([]coachapi.BaselineFileEntry, len(f.entries))
	copy(out, f.entries)
	return out, nil
}

func (f *fakeTreeSource) ReadFile(_ context.Context, _, _, ref, path string) ([]byte, string, error) {
	f.readCalls++
	f.lastReadRef = ref
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

func baselineRSAKey() []byte {
	GinkgoHelper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	Expect(err).NotTo(HaveOccurred())
	block := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}
	return pem.EncodeToMemory(block)
}

// newGitHubBaselineTreeFixture builds a fakegithub Contents + commit graph that
// GitHubBaselineTreeSource can walk end-to-end (resolve → list → read).
// files keys must be top-level basenames (no nested paths).
func newGitHubBaselineTreeFixture(objectSHA string, files map[string][]byte) *fakegithub.Fixture {
	GinkgoHelper()
	fx := fakegithub.NewFixture("handler-github-tree-fixture")
	fx.Installation.Installations[42] = fakegithub.InstallationEntry{
		Token: "handler-install-token", Scenario: fakegithub.ScenarioOK,
	}
	fx.Installation.RepoMappings["acme/widgets"] = fakegithub.RepoInstallationEntry{
		InstallationID: 42, Scenario: fakegithub.ScenarioOK,
	}
	fx.Repos.Repos["acme/widgets"] = fakegithub.RepoMetaEntry{
		DefaultBranch: "main", Scenario: fakegithub.ScenarioOK,
	}
	fx.Repos.Commits["acme/widgets/main"] = fakegithub.CommitEntry{
		SHA: objectSHA, Scenario: fakegithub.ScenarioOK,
	}
	fx.Repos.Commits["acme/widgets/"+objectSHA] = fakegithub.CommitEntry{
		SHA: objectSHA, Scenario: fakegithub.ScenarioOK,
	}

	rootKey := "acme/widgets/" + objectSHA
	rootEntries := make([]fakegithub.DirEntry, 0, len(files))
	i := 0
	for path, body := range files {
		Expect(path).NotTo(ContainSubstring("/"), "fixture helper supports top-level paths only")
		i++
		blob := fmt.Sprintf("blob%d", i)
		fx.Contents.Files[rootKey+"/"+path] = fakegithub.FileEntry{
			Content: body, SHA: blob, Scenario: fakegithub.ScenarioOK,
		}
		rootEntries = append(rootEntries, fakegithub.DirEntry{
			Name: path, Type: "file", SHA: blob, Size: len(body),
		})
	}
	// Root dir listing doubles as the parent listing for top-level ReadFile symlink checks.
	fx.Contents.Dirs[rootKey] = rootEntries
	return &fx
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
			// Seed rubrics also handler-driven. Fixture yields hidden_input_mutation
			// signals, so both rubrics must appear as handler-sourced calls.
			Expect(names).To(ContainElement(rubrics.IDChangeCohesion),
				"change_cohesion must run via agentloop.Call(handler, …)")
			Expect(names).To(ContainElement(rubrics.IDHiddenMutationContextualization),
				"hidden_mutation_contextualization must run via agentloop when deterministic signals exist")
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

		It("fails the job when MaxTotalBytes is exceeded with an actionable too-large error", func() {
			// Fixture supported files total more than a few bytes; a 1-byte budget
			// must fail at list time before any analysis.
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "smoke-owner",
				SmokeRepoName:    "smoke-repo",
				MaxTotalBytes:    1,
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
				"byte-budget path must wrap githubingest.ErrTooLarge; got %v", err)
			Expect(err.Error()).To(Or(
				ContainSubstring("budget"),
				ContainSubstring("too large"),
				ContainSubstring("exceeds"),
				ContainSubstring("byte"),
				ContainSubstring("MaxTotalBytes"),
			))
			Expect(w.findings).To(BeEmpty(), "byte-budget failure must not persist findings")
		})
	})

	When("the fixture tree mixes supported and unsupported extensions", func() {
		It("analyzes only semantics-supported paths (.go, .ts, .tsx) and skips the rest", func() {
			root := GinkgoT().TempDir()
			// Supported languages currently in the pkg/semantics registry.
			Expect(os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(root, "util.ts"), []byte("export const n = 1;\n"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(root, "Widget.tsx"), []byte("export const W = () => null;\n"), 0o644)).To(Succeed())
			// Unsupported: must not appear as semantics_analyze targets.
			Expect(os.WriteFile(filepath.Join(root, "notes.md"), []byte("# docs\n"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(root, "script.py"), []byte("print('no')\n"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(root, "data.json"), []byte(`{"a":1}`+"\n"), 0o644)).To(Succeed())

			var observed *agentloop.Loop
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: root,
				SmokeRepoOwner:   "lang-owner",
				SmokeRepoName:    "lang-repo",
				Gateway:          modelgateway.NewStubGateway(),
				ObserveLoop:      func(loop *agentloop.Loop) { observed = loop },
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "lang-owner",
				RepoName:  "lang-repo",
			}), w)
			Expect(err).NotTo(HaveOccurred())
			Expect(completion).NotTo(BeNil())
			Expect(observed).NotTo(BeNil())

			analyzed := map[string]struct{}{}
			for _, c := range observed.Calls() {
				if c.Source != agentloop.CallSourceHandler || c.Name != agentloop.ToolSemanticsAnalyze {
					continue
				}
				var args struct {
					Path string `json:"path"`
				}
				Expect(json.Unmarshal(c.Args, &args)).To(Succeed(), "semantics_analyze args must be JSON with path")
				analyzed[args.Path] = struct{}{}
			}
			Expect(analyzed).To(HaveKey("main.go"))
			Expect(analyzed).To(HaveKey("util.ts"))
			Expect(analyzed).To(HaveKey("Widget.tsx"))
			Expect(analyzed).NotTo(HaveKey("notes.md"), "markdown is outside the semantics language registry")
			Expect(analyzed).NotTo(HaveKey("script.py"), "python is outside the semantics language registry")
			Expect(analyzed).NotTo(HaveKey("data.json"), "json is outside the semantics language registry")
			Expect(analyzed).To(HaveLen(3), "exactly the three supported-language files must be analyzed")
		})
	})

	When("job params do not match the configured smoke fixture owner/name pair", func() {
		It("does not walk the smoke fixture and fails closed without a TreeSource", func() {
			// Fixture is configured, but params name a different repo — Story 3
			// smoke path is operator-paired only; mismatch must not silently use it.
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "smoke-owner",
				SmokeRepoName:    "smoke-repo",
				// TreeSource intentionally nil: production would use GitHub here.
				Gateway: modelgateway.NewStubGateway(),
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "other-owner",
				RepoName:  "other-repo",
			}), w)
			Expect(completion).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(Or(
				ContainSubstring("no tree source"),
				ContainSubstring("not the smoke fixture"),
				ContainSubstring("TreeSource"),
			), "mismatch must fail closed rather than using the fixture; got %v", err)
			Expect(w.findings).To(BeEmpty())
		})

		It("routes a non-smoke pair through TreeSource instead of the local fixture", func() {
			const resolved = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
			src := &fakeTreeSource{
				resolvedSHA: resolved,
				entries:     []coachapi.BaselineFileEntry{{Path: "only.go", Size: 20}},
				contents: map[string][]byte{
					"only.go": []byte("package only\n\nfunc F() {}\n"),
				},
			}
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "smoke-owner",
				SmokeRepoName:    "smoke-repo",
				TreeSource:       src,
				Gateway:          modelgateway.NewStubGateway(),
			})

			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "acme",
				RepoName:  "widgets",
			}), newCaptureWriter())
			Expect(err).NotTo(HaveOccurred())
			Expect(completion).NotTo(BeNil())
			Expect(completion.CommitSHA).To(Equal(resolved),
				"non-smoke pair must use TreeSource (resolved SHA), not local-fixture")
			Expect(completion.CommitSHA).NotTo(Equal("local-fixture"))
			Expect(src.resolveCalls).To(BeNumerically(">=", 1))
			Expect(src.listCalls).To(BeNumerically(">=", 1))
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

	When("ListFiles succeeds but ReadFile fails mid-fetch", func() {
		It("fails with ErrNotFound and persists no findings", func() {
			src := &fakeTreeSource{
				resolvedSHA: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				entries:     []coachapi.BaselineFileEntry{{Path: "main.go", Size: 20}},
				readErr:     githubingest.ErrNotFound,
			}
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				TreeSource: src,
				Gateway:    modelgateway.NewStubGateway(),
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "acme",
				RepoName:  "widgets",
				Ref:       "main",
			}), w)
			Expect(completion).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, githubingest.ErrNotFound)).To(BeTrue(),
				"mid-fetch ReadFile not-found must remain errors.Is-compatible; got %v", err)
			Expect(src.listCalls).To(BeNumerically(">=", 1))
			Expect(src.readCalls).To(BeNumerically(">=", 1),
				"handler must attempt ReadFile after a successful ListFiles")
			Expect(w.findings).To(BeEmpty(), "mid-fetch failure must not persist findings")
		})

		It("fails with ErrAuth and persists no findings", func() {
			src := &fakeTreeSource{
				resolvedSHA: "cccccccccccccccccccccccccccccccccccccccc",
				entries:     []coachapi.BaselineFileEntry{{Path: "main.go", Size: 20}},
				readErr:     githubingest.ErrAuth,
			}
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				TreeSource: src,
				Gateway:    modelgateway.NewStubGateway(),
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "acme",
				RepoName:  "private",
			}), w)
			Expect(completion).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, githubingest.ErrAuth)).To(BeTrue(),
				"mid-fetch ReadFile auth failure must remain errors.Is-compatible; got %v", err)
			Expect(src.readCalls).To(BeNumerically(">=", 1))
			Expect(w.findings).To(BeEmpty())
		})
	})

	When("the handler uses GitHubBaselineTreeSource against fake GitHub Contents", func() {
		It("completes a baseline via real ListFiles/ReadFile/ResolveCommitSHA, not a tree double", func() {
			const objectSHA = "0123456789abcdef0123456789abcdef01234567"
			// Include a hidden_input_mutation signal so agentloop drives both seed rubrics.
			mutate := []byte(`package main

type C struct{ N string }

func Mut(c *C, n string) { c.N = n }
`)
			plain := []byte("package main\n\nfunc main() {}\n")
			fx := newGitHubBaselineTreeFixture(objectSHA, map[string][]byte{
				"main.go":   plain,
				"mutate.go": mutate,
				"notes.md":  []byte("# ignored\n"),
			})
			server := fakegithub.NewServer(fx)
			DeferCleanup(server.Close)

			reader, err := githubingest.NewGitHubFileReader(githubingest.GitHubAppConfig{
				AppID:          12345,
				InstallationID: 42,
				PrivateKey:     baselineRSAKey(),
				BaseURL:        server.URL(),
			})
			Expect(err).NotTo(HaveOccurred())

			var observed *agentloop.Loop
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				TreeSource: &coachapi.GitHubBaselineTreeSource{Reader: reader},
				Gateway:    modelgateway.NewStubGateway(),
				ObserveLoop: func(loop *agentloop.Loop) {
					observed = loop
				},
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "acme",
				RepoName:  "widgets",
				Ref:       "main",
			}), w)
			Expect(err).NotTo(HaveOccurred(),
				"GitHubBaselineTreeSource must drive handler end-to-end against fakegithub Contents")
			Expect(completion).NotTo(BeNil())
			Expect(completion.CommitSHA).To(Equal(objectSHA),
				"commit_sha must come from ResolveCommitSHA on the real GitHub adapter")
			Expect(completion.CommitSHA).NotTo(Equal("main"))
			Expect(completion.CommitSHA).NotTo(Equal("local-fixture"))

			Expect(observed).NotTo(BeNil())
			names := handlerSourcedNames(observed.Calls())
			Expect(names).To(ContainElement(agentloop.ToolSemanticsAnalyze))
			Expect(names).To(ContainElement(agentloop.ToolCodeSignalReport))

			analyzed := map[string]struct{}{}
			for _, c := range observed.Calls() {
				if c.Source != agentloop.CallSourceHandler || c.Name != agentloop.ToolSemanticsAnalyze {
					continue
				}
				var args struct {
					Path string `json:"path"`
				}
				Expect(json.Unmarshal(c.Args, &args)).To(Succeed())
				analyzed[args.Path] = struct{}{}
			}
			Expect(analyzed).To(HaveKey("main.go"))
			Expect(analyzed).To(HaveKey("mutate.go"))
			Expect(analyzed).NotTo(HaveKey("notes.md"),
				"GitHubBaselineTreeSource must filter to semantics-supported extensions")

			var det int
			for _, f := range w.findings {
				if f.Source == coachapi.FindingSourceDeterministic {
					det++
				}
			}
			Expect(det).To(BeNumerically(">=", 1),
				"Contents-backed tree must produce deterministic findings from mutate.go")
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

	When("rubric judgment fails schema validation after bounded retries", func() {
		It("still completes with deterministic findings and schema diagnostics, without source=agent findings", func() {
			// Story 5 at the handler boundary: schema-invalid model output is a
			// diagnostic, not a failed job, and must not suppress deterministic rows.
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "smoke-owner",
				SmokeRepoName:    "smoke-repo",
				Gateway: modelgateway.NewStubGateway(modelgateway.StubOptions{
					JudgeErr: modelgateway.NewValidationError("judgment missing required field: confidence"),
				}),
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "smoke-owner",
				RepoName:  "smoke-repo",
			}), w)
			Expect(err).NotTo(HaveOccurred(),
				"schema-validation degrade must not fail the job (Story 5); got %v", err)
			Expect(completion).NotTo(BeNil())

			var det int
			for _, f := range w.findings {
				if f.Source == coachapi.FindingSourceDeterministic {
					det++
				}
				Expect(f.Source).NotTo(Equal(coachapi.FindingSourceAgent),
					"schema-invalid judgments must not become source=agent findings")
			}
			Expect(det).To(BeNumerically(">=", 1),
				"deterministic findings must survive schema-validation degrade")
			Expect(w.diagnostics).NotTo(BeEmpty(),
				"schema-validation degrade must record JobDiagnostic entries")

			var sawSchemaDiag bool
			for _, d := range w.diagnostics {
				if strings.Contains(strings.ToLower(d.Message), "schema") ||
					strings.Contains(d.Message, "confidence") ||
					strings.Contains(d.Message, "validation") ||
					strings.HasPrefix(d.Scope, "rubric:") {
					sawSchemaDiag = true
				}
			}
			Expect(sawSchemaDiag).To(BeTrue(),
				"diagnostics should describe schema/validation failure from rubric degrade envelopes; got %#v", w.diagnostics)
		})
	})

	When("client-supplied clone URLs appear in stored job params", func() {
		// HTTP DisallowUnknownFields rejection of git_url/clone_url is owned by
		// server_acceptance_test (Task 2). This slice owns the worker-side permanent
		// reject through the handler's production params parser (parseBaselineParams).
		It("rejects git_url via the handler production params parser", func() {
			raw := []byte(`{"repo_owner":"acme","repo_name":"widgets","git_url":"https://evil.example/x.git"}`)
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "acme",
				SmokeRepoName:    "widgets",
				Gateway:          modelgateway.NewStubGateway(),
			})
			job := baselineJob(coachapi.RepoBaselineScanParams{RepoOwner: "acme", RepoName: "widgets"})
			job.Params = raw
			completion, err := h(context.Background(), job, newCaptureWriter())
			Expect(completion).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(And(
				ContainSubstring("git_url"),
				ContainSubstring("not allowed"),
			), "handler must reject sneaked git_url via production parser; got %v", err)
		})

		It("rejects clone_url via the handler production params parser", func() {
			raw := []byte(`{"repo_owner":"acme","repo_name":"widgets","clone_url":"https://evil.example/x.git"}`)
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "acme",
				SmokeRepoName:    "widgets",
				Gateway:          modelgateway.NewStubGateway(),
			})
			job := baselineJob(coachapi.RepoBaselineScanParams{RepoOwner: "acme", RepoName: "widgets"})
			job.Params = raw
			completion, err := h(context.Background(), job, newCaptureWriter())
			Expect(completion).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(And(
				ContainSubstring("clone_url"),
				ContainSubstring("not allowed"),
			), "handler must reject sneaked clone_url via production parser; got %v", err)
		})
	})

	When("the supported-language tree has more than 50 files", func() {
		It("completes deterministic findings without agentloop max_tool_calls budget exhaustion", func() {
			// DefaultMaxToolCalls is 50; one semantics_analyze per file + codesignal +
			// rubrics exceeds that for any real repo. Fixture must be >50 files.
			root := GinkgoT().TempDir()
			const n = 55
			for i := 0; i < n; i++ {
				// Tiny non-empty Go files (local fixture rejects empty content).
				name := filepath.Join(root, fmt.Sprintf("f%02d.go", i))
				content := fmt.Sprintf("package p%d\n\nfunc F%d() {}\n", i, i)
				Expect(os.WriteFile(name, []byte(content), 0o644)).To(Succeed())
			}
			// One file that yields a deterministic signal so we can assert findings,
			// not merely "handler returned nil error".
			signalFile := filepath.Join(root, "mutate.go")
			Expect(os.WriteFile(signalFile, []byte(`package p

type C struct{ N string }

func Mut(c *C, n string) { c.N = n }
`), 0o644)).To(Succeed())

			var observed *agentloop.Loop
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: root,
				SmokeRepoOwner:   "big-owner",
				SmokeRepoName:    "big-repo",
				Gateway:          modelgateway.NewStubGateway(),
				ObserveLoop:      func(loop *agentloop.Loop) { observed = loop },
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "big-owner",
				RepoName:  "big-repo",
			}), w)
			Expect(err).NotTo(HaveOccurred(),
				"repos with >50 supported files must not fail with ErrBudgetExceeded/max_tool_calls")
			Expect(completion).NotTo(BeNil())
			Expect(errors.Is(err, agentloop.ErrBudgetExceeded)).To(BeFalse())

			var det int
			for _, f := range w.findings {
				if f.Source == coachapi.FindingSourceDeterministic {
					det++
				}
			}
			Expect(det).To(BeNumerically(">=", 1),
				"deterministic codesignal findings must be produced for the large tree")

			Expect(observed).NotTo(BeNil())
			Expect(observed.Budget().MaxToolCalls).To(BeNumerically(">", agentloop.DefaultMaxToolCalls),
				"baseline loop budget must scale above DefaultMaxToolCalls for large trees")
			analyzeCalls := 0
			for _, c := range observed.Calls() {
				if c.Source == agentloop.CallSourceHandler && c.Name == agentloop.ToolSemanticsAnalyze {
					analyzeCalls++
				}
			}
			Expect(analyzeCalls).To(BeNumerically(">=", n),
				"handler must drive semantics_analyze for each supported file")
		})
	})

	When("a GitHub-backed tree source resolves the analyzed commit", func() {
		It("records Completion.CommitSHA as the resolved object SHA, not the branch name or HEAD", func() {
			const resolved = "0123456789abcdef0123456789abcdef01234567"
			src := &fakeTreeSource{
				resolvedSHA: resolved,
				entries: []coachapi.BaselineFileEntry{
					{Path: "main.go", Size: 20},
				},
				contents: map[string][]byte{
					"main.go": []byte("package main\n\nfunc main() {}\n"),
				},
			}
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				TreeSource: src,
				Gateway:    modelgateway.NewStubGateway(),
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "acme",
				RepoName:  "widgets",
				Ref:       "main",
			}), w)
			Expect(err).NotTo(HaveOccurred())
			Expect(completion).NotTo(BeNil())
			Expect(completion.CommitSHA).To(Equal(resolved),
				"commit_sha must be the resolved commit object SHA, not the branch ref")
			Expect(completion.CommitSHA).NotTo(Equal("main"))
			Expect(completion.CommitSHA).NotTo(Equal("HEAD"))
			Expect(src.resolveCalls).To(BeNumerically(">=", 1))
			// List/Read must use the resolved SHA as the ref once resolved.
			Expect(src.lastListRef).To(Equal(resolved))
			Expect(src.lastReadRef).To(Equal(resolved))
		})

		It("resolves an empty ref to a commit object SHA rather than inventing HEAD", func() {
			const resolved = "fedcba9876543210fedcba9876543210fedcba98"
			src := &fakeTreeSource{
				resolvedSHA: resolved,
				entries:     []coachapi.BaselineFileEntry{{Path: "a.go", Size: 10}},
				contents:    map[string][]byte{"a.go": []byte("package a\n")},
			}
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				TreeSource: src,
				Gateway:    modelgateway.NewStubGateway(),
			})
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "acme",
				RepoName:  "widgets",
				// Ref intentionally empty → default branch tip, not literal "HEAD".
			}), newCaptureWriter())
			Expect(err).NotTo(HaveOccurred())
			Expect(completion.CommitSHA).To(Equal(resolved))
			Expect(completion.CommitSHA).NotTo(Equal("HEAD"))
			Expect(src.lastResolveRef).To(Equal(""), "handler must pass empty ref through to ResolveCommitSHA")
		})
	})

	When("judgment fails hard after deterministic analysis (not a gateway-unavailable envelope)", func() {
		It("still completes with deterministic findings already written and a judgment diagnostic", func() {
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "smoke-owner",
				SmokeRepoName:    "smoke-repo",
				Gateway:          modelgateway.NewStubGateway(),
				ConfigureLoop: func(loop *agentloop.Loop) {
					// Replace seed rubric tools with hard failures (plain error,
					// not Story 5 gateway-unavailable diagnostic envelope).
					hard := func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
						return nil, errors.New("injected hard judgment failure")
					}
					Expect(loop.Register(agentloop.ToolSpec{
						Name:    rubrics.IDHiddenMutationContextualization,
						Handler: hard,
					})).To(Succeed())
					Expect(loop.Register(agentloop.ToolSpec{
						Name:    rubrics.IDChangeCohesion,
						Handler: hard,
					})).To(Succeed())
				},
			})

			w := newCaptureWriter()
			completion, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "smoke-owner",
				RepoName:  "smoke-repo",
			}), w)
			Expect(err).NotTo(HaveOccurred(),
				"hard non-cancel judgment error must not FailJob after deterministic analysis (Story 5)")
			Expect(completion).NotTo(BeNil())

			var det int
			for _, f := range w.findings {
				if f.Source == coachapi.FindingSourceDeterministic {
					det++
				}
				Expect(f.Source).NotTo(Equal(coachapi.FindingSourceAgent),
					"hard judgment failure must not leave partial source=agent findings")
			}
			Expect(det).To(BeNumerically(">=", 1),
				"deterministic findings must already be InsertFindings'd before judgment hard-fails")
			Expect(w.diagnostics).NotTo(BeEmpty(), "hard judgment failure must record a JobDiagnostic")
			var sawJudgmentDiag bool
			for _, d := range w.diagnostics {
				if strings.Contains(d.Message, "judgment") || strings.Contains(d.Scope, "judgment") ||
					strings.Contains(d.Message, "injected hard") || strings.Contains(d.Scope, "rubric") {
					sawJudgmentDiag = true
				}
			}
			Expect(sawJudgmentDiag).To(BeTrue(), "diagnostic should describe the judgment-phase failure")
		})

		It("still aborts when the owning context is canceled during judgment", func() {
			ctx, cancel := context.WithCancel(context.Background())
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "smoke-owner",
				SmokeRepoName:    "smoke-repo",
				Gateway:          modelgateway.NewStubGateway(),
				ConfigureLoop: func(loop *agentloop.Loop) {
					Expect(loop.Register(agentloop.ToolSpec{
						Name: rubrics.IDHiddenMutationContextualization,
						Handler: func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
							cancel()
							return nil, context.Canceled
						},
					})).To(Succeed())
					Expect(loop.Register(agentloop.ToolSpec{
						Name: rubrics.IDChangeCohesion,
						Handler: func(_ context.Context, _ json.RawMessage) (json.RawMessage, error) {
							return nil, context.Canceled
						},
					})).To(Succeed())
				},
			})

			completion, err := h(ctx, baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "smoke-owner",
				RepoName:  "smoke-repo",
			}), newCaptureWriter())
			Expect(completion).To(BeNil())
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, context.Canceled)).To(BeTrue(),
				"context.Canceled during judgment must still abort the job; got %v", err)
		})
	})

	When("a successful smoke baseline is completed through MemoryStore", func() {
		It("assembles a GetReport with commit_sha, source-tagged findings, versions.rubrics, and analyzer", func() {
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "smoke-owner",
				SmokeRepoName:    "smoke-repo",
				Gateway:          modelgateway.NewStubGateway(),
			})
			job := baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "smoke-owner",
				RepoName:  "smoke-repo",
				Ref:       "main",
			})
			store, w := newMemoryFencedWriter(job)
			completion, err := h(context.Background(), job, w)
			Expect(err).NotTo(HaveOccurred())
			Expect(completion).NotTo(BeNil())

			lease := w.Lease()
			Expect(store.CompleteJob(context.Background(), lease.JobID, lease.WorkerID, lease.Attempt, *completion)).To(Succeed())

			report, err := store.GetReport(context.Background(), job.ID)
			Expect(err).NotTo(HaveOccurred())
			Expect(report.CommitSHA).To(Equal("local-fixture"))
			Expect(report.Versions.Analyzer).NotTo(BeEmpty())
			Expect(report.Versions.Rubrics).NotTo(BeEmpty())
			Expect(report.Versions.Rubrics).To(HaveKey(rubrics.IDHiddenMutationContextualization))
			Expect(report.Versions.Rubrics).To(HaveKey(rubrics.IDChangeCohesion))
			Expect(report.Findings).NotTo(BeEmpty())
			var sawDet, sawAgent bool
			for _, f := range report.Findings {
				switch f.Source {
				case coachapi.FindingSourceDeterministic:
					sawDet = true
					Expect(f.RubricID).To(BeNil())
				case coachapi.FindingSourceAgent:
					sawAgent = true
					Expect(f.RubricID).NotTo(BeNil())
				}
			}
			Expect(sawDet).To(BeTrue(), "report must include source=deterministic findings")
			Expect(sawAgent).To(BeTrue(), "report must include source=agent findings from stub judgments")
			Expect(report.Error).To(BeNil())
			Expect(report.ReportVersion).To(Equal(coachapi.ReportVersion1))
		})
	})

	When("the local smoke fixture contains a symlink that points outside the root", func() {
		It("skips the symlink in ListFiles and rejects it on ReadFile", func() {
			root := GinkgoT().TempDir()
			outside := GinkgoT().TempDir()
			secret := filepath.Join(outside, "secret.go")
			Expect(os.WriteFile(secret, []byte("package secret\n\nfunc Leak() {}\n"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(root, "safe.go"), []byte("package safe\n"), 0o644)).To(Succeed())
			Expect(os.Symlink(secret, filepath.Join(root, "leak.go"))).To(Succeed())

			src := &coachapi.LocalFixtureTreeSource{Root: root}
			entries, err := src.ListFiles(context.Background(), "o", "r", "", coachapi.BaselineListOptions{})
			Expect(err).NotTo(HaveOccurred())
			paths := make([]string, 0, len(entries))
			for _, e := range entries {
				paths = append(paths, e.Path)
			}
			Expect(paths).To(ContainElement("safe.go"))
			Expect(paths).NotTo(ContainElement("leak.go"),
				"ListFiles must not surface fixture symlinks (GitHub Contents skips them)")

			_, _, err = src.ReadFile(context.Background(), "o", "r", "", "leak.go")
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, githubingest.ErrUnsupportedContent)).To(BeTrue(),
				"ReadFile must reject symlinks without following them; got %v", err)
		})
	})
})
