package codesignalcli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/codesignalcli/internal/gitfixture"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
	"github.com/lousy-agents/coach/pkg/projectmodel"
)

// snapshotOpeningBackend opens the head snapshot the way a project backend
// does before it builds a model, so the worktree view has an outcome to report.
type snapshotOpeningBackend struct {
	result *ProjectBackendResult
}

func (b snapshotOpeningBackend) Analyze(_ context.Context, req ProjectBackendRequest) (*ProjectBackendResult, error) {
	_, err := req.HeadSnapshot(req.Dir, req.HeadRevision)
	return b.result, err
}

var _ = Describe("applyProjectBackend with a worktree view returns new values", func() {
	var (
		dir      string
		revision string
		project  *ProjectAnalysis
		result   *ProjectBackendResult
		original *projectmodel.Coverage
	)

	BeforeEach(func() {
		dir = gitfixture.Init(GinkgoT())
		gitfixture.CommitFile(GinkgoT(), dir, "go.mod", "module example.com/m\n\ngo 1.25\n")
		revision = gitfixture.CommitFile(GinkgoT(), dir, "pkg/a/a.go", "package a\n")
		target := filepath.Join(GinkgoT().TempDir(), "a.go")
		Expect(os.WriteFile(target, []byte("package a\n"), 0o644)).To(Succeed())
		Expect(os.Remove(filepath.Join(dir, "pkg/a/a.go"))).To(Succeed())
		Expect(os.Symlink(target, filepath.Join(dir, "pkg/a/a.go"))).To(Succeed())

		original = &projectmodel.Coverage{Phase: "go_model_build", Complete: true}
		result = &ProjectBackendResult{HeadCoverage: original}
		config := json.RawMessage(`{"schema_version":"1","roots":["."]}`)
		view := NewWorktreeView(ReadWorktreeStatus(dir, true))
		project = &ProjectAnalysis{
			Policy:       policyref.Discovered("project.json"),
			Language:     "go",
			Config:       config,
			ConfigDigest: projectconfig.Digest(config),
			Backend:      snapshotOpeningBackend{result: result},
			Worktree:     &view,
		}
	})

	It("does not write the worktree diagnostics into the caller's Input.Diagnostics backing array", func() {
		callerSlice := make([]codesignal.Diagnostic, 1, 4)
		callerSlice[0] = codesignal.Diagnostic{Kind: "pre_existing", Message: "must survive"}

		returned, _, err := applyProjectBackend(context.Background(), codesignal.Input{Diagnostics: callerSlice}, codesignal.Options{}, project, dir, revision, "", true)

		Expect(err).NotTo(HaveOccurred())
		Expect(callerSlice[:cap(callerSlice)][1].Kind).To(BeEmpty(),
			"a slot past the caller's length was written: the append aliased Input.Diagnostics")
		Expect(returned.Diagnostics).To(ContainElement(HaveField("Kind", codesignal.DiagKindWorktreeSymlinkSkipped)))
		Expect(returned.Diagnostics[0].Kind).To(Equal("pre_existing"))
	})

	It("marks coverage incomplete on the returned input and leaves the backend's result as it was", func() {
		returned, _, err := applyProjectBackend(context.Background(), codesignal.Input{}, codesignal.Options{}, project, dir, revision, "", true)

		Expect(err).NotTo(HaveOccurred())
		Expect(returned.ProjectCoverage).NotTo(BeNil())
		Expect(returned.ProjectCoverage.Complete).To(BeFalse(), "a symbolic link inside the roots was dropped from the model")
		Expect(result.HeadCoverage).To(BeIdenticalTo(original), "the backend's result was written to")
		Expect(original.Complete).To(BeTrue())
	})
})
