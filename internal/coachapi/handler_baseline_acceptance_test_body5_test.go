package coachapi_test

import (
	"context"

	"errors"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/coachapi"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
	"github.com/lousy-agents/coach/pkg/githubingest"
)

func body_handlerBaselineAcceptanceTest_assemblesAGetReportWithCommitShaSourceTaggedFind_938() {
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
}

func body_handlerBaselineAcceptanceTest_skipsTheSymlinkInListFilesAndRejectsItOnReadFile_985() {
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
}

func body_handlerBaselineAcceptanceTest_listsThemLikeGitHubContentsDoesNotDropPathsStart_1012() {
	root := GinkgoT().TempDir()
	dotDir := filepath.Join(root, ".github", "workflows")
	Expect(os.MkdirAll(dotDir, 0o755)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, "main.go"), []byte("package main\n"), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(dotDir, "ci.ts"), []byte("export const x = 1\n"), 0o644)).To(Succeed())
	Expect(os.WriteFile(filepath.Join(root, ".hidden.go"), []byte("package hidden\n"), 0o644)).To(Succeed())

	src := &coachapi.LocalFixtureTreeSource{Root: root}
	entries, err := src.ListFiles(context.Background(), "o", "r", "", coachapi.BaselineListOptions{})
	Expect(err).NotTo(HaveOccurred())
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		paths = append(paths, e.Path)
	}
	Expect(paths).To(ContainElements("main.go", ".github/workflows/ci.ts", ".hidden.go"),
		"ListFiles must include supported sources under top-level dot paths for GitHub parity; got %v", paths)
}

func body_handlerBaselineAcceptanceTest_admitsAllOfThemAndOneFileOverMaxFilesTripsTheBud_1033() {
	root := GinkgoT().TempDir()
	for i := 0; i < 3; i++ {
		Expect(os.WriteFile(filepath.Join(root, fmt.Sprintf("f%d.go", i)), []byte("package f\n"), 0o644)).To(Succeed())
	}

	src := &coachapi.LocalFixtureTreeSource{Root: root}
	entries, err := src.ListFiles(context.Background(), "o", "r", "", coachapi.BaselineListOptions{MaxFiles: 3})
	Expect(err).NotTo(HaveOccurred(), "exactly MaxFiles eligible files must be admitted, not rejected")
	Expect(entries).To(HaveLen(3))

	Expect(os.WriteFile(filepath.Join(root, "f3.go"), []byte("package f\n"), 0o644)).To(Succeed())
	_, err = src.ListFiles(context.Background(), "o", "r", "", coachapi.BaselineListOptions{MaxFiles: 3})
	Expect(err).To(HaveOccurred())
	Expect(errors.Is(err, githubingest.ErrTooLarge)).To(BeTrue(), "one file over MaxFiles must trip the budget; got %v", err)
}
