package baseline_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"

	"github.com/lousy-agents/coach/internal/coachapi/baseline"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/modelgateway"
)

var _ = Describe("repo_baseline_scan over the committed baseline fixture", func() {
	When("the handler scans testdata/baseline_fixture exactly as committed", func() {
		It("reports source=deterministic hidden-input mutations from the fixture's own widget/update.go and widget/reset.go", func() {
			_, thisFile, _, ok := runtime.Caller(0)
			Expect(ok).To(BeTrue())
			committed := filepath.Join(filepath.Dir(thisFile), "..", "testdata", "baseline_fixture")

			h := baseline.NewScanHandler(baseline.ScanConfig{
				SmokeFixturePath: committed,
				SmokeRepoOwner:   "smoke-owner",
				SmokeRepoName:    "smoke-repo",
				Gateway:          modelgateway.NewStubGateway(),
			})
			cap := newCaptureWriter()
			_, err := h(context.Background(), baselineJob(coachapi.RepoBaselineScanParams{
				RepoOwner: "smoke-owner",
				RepoName:  "smoke-repo",
				Ref:       "main",
			}), cap)
			Expect(err).NotTo(HaveOccurred())

			var rulePaths []string
			for _, f := range cap.findings {
				if f.Source != coachapi.FindingSourceDeterministic {
					continue
				}
				var sig struct {
					RuleID string `json:"rule_id"`
					Path   string `json:"path"`
				}
				Expect(json.Unmarshal(f.Payload, &sig)).To(Succeed())
				rulePaths = append(rulePaths, sig.RuleID+" "+sig.Path)
			}
			Expect(rulePaths).To(ContainElements(
				"state.hidden_input_mutation widget/update.go",
				"state.hidden_input_mutation widget/reset.go",
			), "the committed fixture must itself trigger the hidden-input signals the handler specs depend on")
		})
	})
})
