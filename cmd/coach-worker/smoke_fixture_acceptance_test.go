package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
)

var _ = Describe("credential-free smoke fixture", func() {
	When("the mounted smoke-repo widget files are value copies", func() {
		It("still records a source=deterministic finding without rewriting the committed tree", func() {
			_, thisFile, _, ok := runtime.Caller(0)
			Expect(ok).To(BeTrue())
			root := filepath.Join(filepath.Dir(thisFile), "..", "..", "deploy", "compose", "platform", "fixtures", "smoke-repo")
			committed, err := os.ReadFile(filepath.Join(root, "widget", "update_test.go"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(committed)).To(ContainSubstring("func UpdateName(cfg Config, name string) Config"))
			Expect(string(committed)).NotTo(ContainSubstring("*Config"))

			h, err := buildJobHandler(Config{
				SmokeFixturePath: root,
				SmokeRepoOwner:   "coach-smoke",
				SmokeRepoName:    "fixture-repo",
			})
			Expect(err).NotTo(HaveOccurred())

			job := coachapi.Job{
				ID:     "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee",
				Kind:   coachapi.JobKindRepoBaselineScan,
				Params: []byte(`{"repo_owner":"coach-smoke","repo_name":"fixture-repo"}`),
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

			w := &findingWriter{storeJobWriter: storeJobWriter{store: store, lease: lease}}
			_, err = h(context.Background(), job, w)
			Expect(err).NotTo(HaveOccurred())

			var deterministic int
			for _, f := range w.findings {
				if f.Source == coachapi.FindingSourceDeterministic {
					deterministic++
				}
			}
			Expect(deterministic).To(BeNumerically(">", 0))

			after, err := os.ReadFile(filepath.Join(root, "widget", "update_test.go"))
			Expect(err).NotTo(HaveOccurred())
			Expect(after).To(Equal(committed))
		})
	})
})

type findingWriter struct {
	storeJobWriter
	findings []coachapi.JobFinding
}

func (w *findingWriter) InsertFindings(ctx context.Context, findings []coachapi.JobFinding) error {
	w.findings = append(w.findings, findings...)
	return w.storeJobWriter.InsertFindings(ctx, findings)
}
