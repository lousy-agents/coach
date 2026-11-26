package evaldetect_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
)

const edgeRule = "architecture.required_edge_absent"

var _ = Describe("the required-edge check (design §4, `architecture.layer_violation`: a missing required edge is a G2 absence)", func() {
	const module = "module example.com/shop\n\ngo 1.22\n"
	edge := evaldetect.Edge{From: "internal/api/entry", To: "internal/service"}

	check := func(files map[string]string, edges ...evaldetect.Edge) ([]evaldetect.Finding, error) {
		return evaldetect.RequiredEdgeAbsences(writeTree(GinkgoT().TempDir(), files), edges)
	}

	It("reports nothing when a non-test file of the from package imports the to package (AC-31)", func() {
		findings, err := check(map[string]string{
			"go.mod":                      module,
			"internal/api/entry/entry.go": "package entry\n\nimport _ \"example.com/shop/internal/service\"\n",
			"internal/service/svc.go":     "package service\n",
		}, edge)

		Expect(err).NotTo(HaveOccurred())
		Expect(findings).To(BeEmpty())
	})

	DescribeTable("reports the edge as absent, naming the rule and the edge",
		func(files map[string]string) {
			findings, err := check(files, edge)

			Expect(err).NotTo(HaveOccurred())
			Expect(findings).To(ConsistOf(evaldetect.Finding{
				RuleID: edgeRule, Path: "internal/api/entry", Subject: edgeRule + ":internal/api/entry->internal/service",
			}))
		},
		Entry("when the from package imports nothing (AC-31)", map[string]string{
			"go.mod":                      module,
			"internal/api/entry/entry.go": "package entry\n",
			"internal/service/svc.go":     "package service\n",
		}),
		Entry("when only a test file makes the import (AC-31)", map[string]string{
			"go.mod":                           module,
			"internal/api/entry/entry.go":      "package entry\n",
			"internal/api/entry/entry_test.go": "package entry\n\nimport _ \"example.com/shop/internal/service\"\n",
			"internal/service/svc.go":          "package service\n",
		}),
		Entry("when another package makes the import (AC-31)", map[string]string{
			"go.mod":                      module,
			"internal/api/entry/entry.go": "package entry\n",
			"internal/api/other/other.go": "package other\n\nimport _ \"example.com/shop/internal/service\"\n",
			"internal/service/svc.go":     "package service\n",
		}),
		Entry("when the import is of a different package than the edge's (AC-31)", map[string]string{
			"go.mod":                      module,
			"internal/api/entry/entry.go": "package entry\n\nimport _ \"example.com/shop/internal/store\"\n",
			"internal/service/svc.go":     "package service\n",
			"internal/store/store.go":     "package store\n",
		}),
	)

	It("reports each missing edge of several, and only those, in a stable order (AC-31)", func() {
		findings, err := check(map[string]string{
			"go.mod":                      module,
			"internal/api/entry/entry.go": "package entry\n\nimport _ \"example.com/shop/internal/service\"\n",
			"internal/service/svc.go":     "package service\n",
			"internal/store/store.go":     "package store\n",
		}, evaldetect.Edge{From: "internal/service", To: "internal/store"}, edge, evaldetect.Edge{From: "internal/api/entry", To: "internal/store"})

		Expect(err).NotTo(HaveOccurred())
		Expect(findings).To(HaveLen(2))
		Expect(findings[0].Subject).To(Equal(edgeRule + ":internal/api/entry->internal/store"))
		Expect(findings[1].Subject).To(Equal(edgeRule + ":internal/service->internal/store"))
	})

	It("refuses a Go file it cannot parse (AC-31)", func() {
		_, err := check(map[string]string{
			"go.mod":                      module,
			"internal/api/entry/entry.go": "package entry\n\nimport (\n",
		}, edge)

		Expect(err).To(HaveOccurred())
	})
})
