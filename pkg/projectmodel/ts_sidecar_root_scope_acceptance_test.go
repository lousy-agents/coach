package projectmodel_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/projectbridge"
)

// Issue #331 AC-13.
var _ = Describe("Sidecar response root scope accounting", func() {
	When("a raw sidecar response line reports root_scopes for two nested project roots", func() {
		It("decodes each root's candidate/analyzed file counts independently, without summing or collapsing them", func() {
			raw := []byte(`{
				"version": 1,
				"id": 42,
				"root_scopes": [
					{"root": ".", "candidate_files": 12, "analyzed_files": 9},
					{"root": "services/payments", "candidate_files": 4, "analyzed_files": 4}
				],
				"coverage": {"phase": "ts_sidecar_analyze", "complete": true}
			}`)

			var resp projectbridge.Response
			Expect(json.Unmarshal(raw, &resp)).To(Succeed())

			Expect(resp.RootScopes).To(HaveLen(2), "expected both the outer root and the nested/overlapping root to decode as independent entries")
			Expect(resp.RootScopes).To(ContainElement(projectbridge.RootScopeFact{
				Root: ".", CandidateFiles: 12, AnalyzedFiles: 9,
			}), "expected the outer root's own counts to survive decode unchanged")
			Expect(resp.RootScopes).To(ContainElement(projectbridge.RootScopeFact{
				Root: "services/payments", CandidateFiles: 4, AnalyzedFiles: 4,
			}), "expected the nested root's own counts to survive decode unchanged, distinct from the outer root's")
		})
	})
})
