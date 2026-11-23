package codesignalcli

import (
	"context"
	"encoding/json"

	"github.com/lousy-agents/coach/internal/codesignalcli/gitrepo"
	"github.com/lousy-agents/coach/internal/codesignalcli/policyref"
	"github.com/lousy-agents/coach/internal/codesignalcli/projectconfig"
	"github.com/lousy-agents/coach/pkg/codesignal"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("the committed-HEAD provenance diagnostic when the project policy is read from disk", func() {
	provenanceDiagnostics := func(policyPath, rootsJSON string, porcelain ...string) []codesignal.Diagnostic {
		cfg := json.RawMessage(`{"schema_version":"1","roots":` + rootsJSON + `}`)
		project := &ProjectAnalysis{
			Policy:       policyref.Discovered(policyPath),
			Language:     "typescript",
			Config:       cfg,
			ConfigDigest: projectconfig.Digest(cfg),
			Backend:      identityHandoffBackend{result: &ProjectBackendResult{}},
			Status:       WorktreeStatus{Entries: gitrepo.ParseWorktreeStatus(porcelainZ(porcelain...))},
		}
		input, _, err := applyProjectBackend(context.Background(), codesignal.Input{}, codesignal.Options{}, project, ".", "HEAD", "", true)
		Expect(err).NotTo(HaveOccurred())
		var found []codesignal.Diagnostic
		for _, diagnostic := range input.Diagnostics {
			if diagnostic.Kind == codesignal.DiagKindWorktreeReportReflectsCommittedHEAD {
				found = append(found, diagnostic)
			}
		}
		return found
	}

	DescribeTable("a dirty policy file alone does not make the report reflect committed HEAD",
		func(policyPath, rootsJSON, porcelain string) {
			Expect(provenanceDiagnostics(policyPath, rootsJSON, porcelain)).To(BeEmpty())
		},
		Entry("a policy under the roots, edited", ".lousy-agents/coach.json", `["."]`, " M .lousy-agents/coach.json"),
		Entry("a policy outside the roots, edited", ".lousy-agents/coach.json", `["src"]`, " M .lousy-agents/coach.json"),
		Entry("a policy untracked", "project.json", `["."]`, "?? project.json"),
		Entry("a policy at a path no default names", "config/policy.json", `["."]`, " M config/policy.json"),
	)

	DescribeTable("a dirty source file under the roots still makes the report reflect committed HEAD, once",
		func(policyPath, rootsJSON string, porcelain ...string) {
			Expect(provenanceDiagnostics(policyPath, rootsJSON, porcelain...)).To(HaveLen(1))
		},
		Entry("the source alone", ".lousy-agents/coach.json", `["src"]`, " M src/app.ts"),
		Entry("the source beside a dirty policy", ".lousy-agents/coach.json", `["src"]`, " M .lousy-agents/coach.json", " M src/app.ts"),
		Entry("the source beside a dirty policy under the roots", ".lousy-agents/coach.json", `["."]`, " M .lousy-agents/coach.json", " M src/app.ts"),
	)

	It("does not report a dirty file outside the roots beside a dirty policy", func() {
		Expect(provenanceDiagnostics(".lousy-agents/coach.json", `["src"]`, " M .lousy-agents/coach.json", " M docs/notes.md")).To(BeEmpty())
	})
})
