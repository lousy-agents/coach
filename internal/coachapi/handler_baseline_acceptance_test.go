package coachapi_test

import (
	"context"

	"encoding/json"

	"errors"
	"os"
	"path/filepath"
	"regexp"

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

var _ coachapi.BaselineJobWriter = (*captureWriter)(nil)

// memoryFencedWriter is a leaseWriter-equivalent over MemoryStore so acceptance
// exercises the real fenced InsertFindings/InsertDiagnostics path.
type memoryFencedWriter struct {
	store *coachapi.MemoryStore
	lease coachapi.ClaimLease
}

var _ coachapi.BaselineJobWriter = (*memoryFencedWriter)(nil)

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

var _ = Describe("repo_baseline_scan job handler", func() {
	When("worker is configured with a local smoke fixture path", func() {
		It("completes a baseline via agentloop against the fixture and records deterministic findings", func() {
			body_handlerBaselineAcceptanceTest_completesABaselineViaAgentloopAgainstTheFixtureA_70()
		})

		It("stamps completion times from an injected clock rather than the wall clock", func() {
			stamp := time.Date(2026, 9, 13, 15, 4, 5, 0, time.UTC)
			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "smoke-owner",
				SmokeRepoName:    "smoke-repo",
				Gateway:          modelgateway.NewStubGateway(),
				Now:              func() time.Time { return stamp },
			})
			job := baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "smoke-owner",
				RepoName:  "smoke-repo",
				Ref:       "main",
			})
			_, w := newMemoryFencedWriter(job)
			completion, err := h(context.Background(), job, w)
			Expect(err).NotTo(HaveOccurred())
			Expect(completion).NotTo(BeNil())
			Expect(completion.FinishedAt).To(Equal(stamp),
				"injected Now should stamp FinishedAt: got %v want %v", completion.FinishedAt, stamp)
			Expect(completion.GeneratedAt).To(Equal(stamp),
				"injected Now should stamp GeneratedAt: got %v want %v", completion.GeneratedAt, stamp)
		})

		It("persists distinct agent payload_hash values for multiple hidden_mutation signals", func() {
			body_handlerBaselineAcceptanceTest_persistsDistinctAgentPayloadHashValuesForMultipl_162()
		})

		It("records handler-sourced semantics_analyze and codesignal_report calls on the loop", func() {
			body_handlerBaselineAcceptanceTest_recordsHandlerSourcedSemanticsAnalyzeAndCodesign_218()
		})
	})

	When("the repository exceeds the configured size budget", func() {
		It("fails the job with an actionable too-large error", func() {
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
			body_handlerBaselineAcceptanceTest_analyzesOnlySemanticsSupportedPathsGoTsTsxAndSki_312()
		})
	})

	When("job params do not match the configured smoke fixture owner/name pair", func() {
		It("does not walk the smoke fixture and fails closed without a TreeSource", func() {

			h := coachapi.NewRepoBaselineScanHandler(coachapi.RepoBaselineScanConfig{
				SmokeFixturePath: baselineFixtureRoot(),
				SmokeRepoOwner:   "smoke-owner",
				SmokeRepoName:    "smoke-repo",
				Gateway:          modelgateway.NewStubGateway(),
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
			body_handlerBaselineAcceptanceTest_completesABaselineViaRealListFilesReadFileResolv_508()
		})
	})

	When("the model gateway is unavailable for judgment", func() {
		It("still completes with deterministic findings and judgment diagnostics", func() {
			body_handlerBaselineAcceptanceTest_stillCompletesWithDeterministicFindingsAndJudgme_594()
		})
	})

	When("rubric judgment fails schema validation after bounded retries", func() {
		It("still completes with deterministic findings and schema diagnostics, without source=agent findings", func() {
			body_handlerBaselineAcceptanceTest_stillCompletesWithDeterministicFindingsAndSchema_640()
		})
	})

	When("client-supplied clone URLs appear in stored job params", func() {

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
			body_handlerBaselineAcceptanceTest_completesDeterministicFindingsWithoutAgentloopMa_728()
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
			}), newCaptureWriter())
			Expect(err).NotTo(HaveOccurred())
			Expect(completion.CommitSHA).To(Equal(resolved))
			Expect(completion.CommitSHA).NotTo(Equal("HEAD"))
			Expect(src.lastResolveRef).To(Equal(""), "handler must pass empty ref through to ResolveCommitSHA")
		})
	})

	When("judgment fails hard after deterministic analysis (not a gateway-unavailable envelope)", func() {
		It("still completes with deterministic findings already written and a judgment diagnostic", func() {
			body_handlerBaselineAcceptanceTest_stillCompletesWithDeterministicFindingsAlreadyWr_850()
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
			body_handlerBaselineAcceptanceTest_assemblesAGetReportWithCommitShaSourceTaggedFind_938()
		})
	})

	When("the local smoke fixture contains a symlink that points outside the root", func() {
		It("skips the symlink in ListFiles and rejects it on ReadFile", func() {
			body_handlerBaselineAcceptanceTest_skipsTheSymlinkInListFilesAndRejectsItOnReadFile_985()
		})
	})

	When("the local smoke fixture has supported sources under a top-level dot directory", func() {
		It("lists them like GitHub Contents (does not drop paths starting with '.')", func() {
			body_handlerBaselineAcceptanceTest_listsThemLikeGitHubContentsDoesNotDropPathsStart_1012()
		})
	})

	When("the local fixture's eligible file count equals MaxFiles exactly", func() {
		It("admits all of them, and one file over MaxFiles trips the budget", func() {
			body_handlerBaselineAcceptanceTest_admitsAllOfThemAndOneFileOverMaxFilesTripsTheBud_1033()
		})
	})

	When("the local fixture's eligible-file total size equals MaxTotalBytes exactly", func() {
		It("admits it, and one byte over trips the budget", func() {
			root := GinkgoT().TempDir()
			content := []byte("package f\n")
			Expect(os.WriteFile(filepath.Join(root, "f.go"), content, 0o644)).To(Succeed())

			src := &coachapi.LocalFixtureTreeSource{Root: root}
			entries, err := src.ListFiles(context.Background(), "o", "r", "", coachapi.BaselineListOptions{MaxTotalBytes: int64(len(content))})
			Expect(err).NotTo(HaveOccurred(), "a fixture whose total size equals MaxTotalBytes exactly must be admitted, not rejected")
			Expect(entries).To(HaveLen(1))

			_, err = src.ListFiles(context.Background(), "o", "r", "", coachapi.BaselineListOptions{MaxTotalBytes: int64(len(content)) - 1})
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, githubingest.ErrTooLarge)).To(BeTrue(), "one byte over MaxTotalBytes must trip the budget; got %v", err)
		})
	})
})
