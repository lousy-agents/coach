package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"runtime"

	"github.com/lousy-agents/coach/internal/coachapi/store/memory"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
)

var _ = Describe("credential-free smoke fixture as committed", func() {
	When("the worker scans the committed deploy/compose smoke-repo", func() {
		It("reports source=deterministic hidden-input mutations from the fixture's own widget/update.go and widget/reset.go", func() {
			_, thisFile, _, ok := runtime.Caller(0)
			Expect(ok).To(BeTrue())
			root := filepath.Join(filepath.Dir(thisFile), "..", "..", "deploy", "compose", "platform", "fixtures", "smoke-repo")

			h, err := buildJobHandler(Config{
				SmokeFixturePath: root,
				SmokeRepoOwner:   "coach-smoke",
				SmokeRepoName:    "fixture-repo",
			})
			Expect(err).NotTo(HaveOccurred())

			job := coachapi.Job{
				ID:     "ffffffff-ffff-ffff-ffff-ffffffffffff",
				Kind:   coachapi.JobKindRepoBaselineScan,
				Params: []byte(`{"repo_owner":"coach-smoke","repo_name":"fixture-repo"}`),
				Status: coachapi.JobStatusRunning,
			}
			store := memory.NewStore()
			Expect(store.CreateJob(context.Background(), coachapi.Job{
				ID: job.ID, Kind: job.Kind, Params: job.Params,
				Status:            coachapi.JobStatusQueued,
				CreatedByProvider: "github", CreatedBySubject: "1", CreatedByLogin: "octocat",
			})).To(Succeed())
			lease, err := store.ClaimJob(context.Background(), job.ID, "w1", storeNow(), storeStale())
			Expect(err).NotTo(HaveOccurred())

			w := &committedFixtureFindingWriter{storeJobWriter: storeJobWriter{store: store, lease: lease}}
			_, err = h(context.Background(), job, w)
			Expect(err).NotTo(HaveOccurred())

			Expect(deterministicRulePaths(w.findings)).To(ContainElements(
				"state.hidden_input_mutation widget/update.go",
				"state.hidden_input_mutation widget/reset.go",
			), "the committed fixture must itself trigger the deterministic signals the smoke report depends on")
		})
	})
})

type committedFixtureFindingWriter struct {
	storeJobWriter
	findings []coachapi.JobFinding
}

func (w *committedFixtureFindingWriter) InsertFindings(ctx context.Context, findings []coachapi.JobFinding) error {
	w.findings = append(w.findings, findings...)
	return w.storeJobWriter.InsertFindings(ctx, findings)
}

func deterministicRulePaths(findings []coachapi.JobFinding) []string {
	GinkgoHelper()
	var out []string
	for _, f := range findings {
		if f.Source != coachapi.FindingSourceDeterministic {
			continue
		}
		var sig struct {
			RuleID string `json:"rule_id"`
			Path   string `json:"path"`
		}
		Expect(json.Unmarshal(f.Payload, &sig)).To(Succeed())
		out = append(out, sig.RuleID+" "+sig.Path)
	}
	return out
}
