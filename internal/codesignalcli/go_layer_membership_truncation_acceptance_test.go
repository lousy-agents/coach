package codesignalcli

import (
	"context"
	"encoding/json"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var goLayerMembershipConfigJSON = json.RawMessage(`{"schema_version":"1","roots":["."],"layers":[{"name":"db","prefixes":["pkg/db"]}]}`)

var _ = Describe("Go layer membership when the project model was truncated", func() {
	var originalBudgets = goProjectBudgets

	BeforeEach(func() {
		DeferCleanup(func() { goProjectBudgets = originalBudgets })
	})

	analyzeLayeredRepo := func() *codesignal.Report {
		dir := gitfixture.Init(GinkgoT())
		gitfixture.CommitFile(GinkgoT(), dir, "go.mod", "module example.com/app\n\ngo 1.25\n")
		gitfixture.CommitFile(GinkgoT(), dir, "pkg/db/db.go", "package db\n")
		gitfixture.CommitFile(GinkgoT(), dir, "pkg/other/a.go", "package other\n")
		sha := gitfixture.CommitFile(GinkgoT(), dir, "pkg/other/b.go", "package other\n")
		files := []gitrepo.SelectedFile{{Path: "pkg/other/b.go", Language: "go", Status: "added"}}
		project := &ProjectAnalysis{
			ConfigPath:   "project.json",
			Language:     "go",
			Config:       goLayerMembershipConfigJSON,
			ConfigDigest: projectconfig.Digest(goLayerMembershipConfigJSON),
			Backend:      NewGoProjectBackend(),
		}
		report, err := AnalyzeBaseline(context.Background(), dir, sha, files, nil, "", codesignal.Coverage{TrackedFilesDiscovered: 4}, project)
		Expect(err).NotTo(HaveOccurred())
		return report
	}

	It("counts every analyzed file when the model is complete", func() {
		report := analyzeLayeredRepo()

		Expect(report.ProjectCoverage.Complete).To(BeTrue())
		Expect(report.LayerMembership).NotTo(BeNil())
		Expect(report.LayerMembership.Head.AnalyzedFiles).To(Equal(3))
		Expect(report.LayerMembership.Head.FilesOutsideLayers).To(Equal(2))
	})

	It("leaves the block out when the input budget dropped files, so the skipped tail is not reported as inside a layer", func() {
		goProjectBudgets.MaxInputFiles = 2

		report := analyzeLayeredRepo()

		Expect(report.ProjectCoverage.Complete).To(BeFalse(), "the budget must actually truncate, or this spec proves nothing")
		Expect(report.LayerMembership).To(BeNil(), "a count over the analyzed head of a truncated model would be a false zero for the skipped tail")
	})
})
