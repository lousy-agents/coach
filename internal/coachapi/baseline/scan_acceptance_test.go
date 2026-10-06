package baseline_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

var _ = Describe("repo_baseline_scan job handler", func() {
	When("worker is configured with a local smoke fixture path", func() {
		It("completes a baseline via agentloop against the fixture and records deterministic findings", func() {
			expectSmokeFixtureBaselineRecordsDeterministicFindings()
		})

		It("stamps completion times from an injected clock rather than the wall clock", func() {
			stamp := time.Date(2026, 9, 13, 15, 4, 5, 0, time.UTC)
			h := baseline.NewScanHandler(baseline.ScanConfig{
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
			expectDistinctAgentPayloadHashesPerHiddenMutation()
		})

		It("records handler-sourced semantics_analyze and codesignal_report calls on the loop", func() {
			expectHandlerSourcedAnalysisCallsRecorded()
		})
	})

	When("the repository exceeds the configured size budget", func() {
		It("fails the job with an actionable too-large error", func() {
			h := baseline.NewScanHandler(baseline.ScanConfig{
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
			h := baseline.NewScanHandler(baseline.ScanConfig{
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
			expectOnlySemanticsSupportedPathsAnalyzed()
		})
	})

	When("job params do not match the configured smoke fixture owner/name pair", func() {
		It("does not walk the smoke fixture and fails closed without a TreeSource", func() {

			h := baseline.NewScanHandler(baseline.ScanConfig{
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
				entries:     []baseline.FileEntry{{Path: "only.go", Size: 20}},
				contents: map[string][]byte{
					"only.go": []byte("package only\n\nfunc F() {}\n"),
				},
			}
			h := baseline.NewScanHandler(baseline.ScanConfig{
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

	When("the supported-language tree has more than 50 files", func() {
		It("completes deterministic findings without agentloop max_tool_calls budget exhaustion", func() {
			expectLargeTreeCompletesWithinToolCallBudget()
		})
	})

	When("a successful smoke baseline is completed through memory.Store", func() {
		It("assembles a GetReport with commit_sha, source-tagged findings, versions.rubrics, and analyzer", func() {
			expectSmokeBaselineAssemblesReport()
		})
	})
})
