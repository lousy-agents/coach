package baseline_test

import (
	"context"
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/coachapi/baseline"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

var _ = Describe("repo_baseline_scan job handler", func() {
	When("the model gateway is unavailable for judgment", func() {
		It("still completes with deterministic findings and judgment diagnostics", func() {
			expectGatewayUnavailableKeepsDeterministicFindings()
		})
	})

	When("rubric judgment fails schema validation after bounded retries", func() {
		It("still completes with deterministic findings and schema diagnostics, without source=agent findings", func() {
			expectSchemaFailureKeepsDeterministicFindings()
		})
	})

	When("judgment fails hard after deterministic analysis (not a gateway-unavailable envelope)", func() {
		It("still completes with deterministic findings already written and a judgment diagnostic", func() {
			expectHardJudgmentFailureKeepsWrittenFindings()
		})

		It("still aborts when the owning context is canceled during judgment", func() {
			ctx, cancel := context.WithCancel(context.Background())
			h := baseline.NewScanHandler(baseline.ScanConfig{
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
})
