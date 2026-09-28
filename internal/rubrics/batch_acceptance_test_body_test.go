package rubrics_test

import (
	"context"
	"encoding/json"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/agentloop"
	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func body_batchAcceptanceTest_returnsThreeValidJudgmentsWithMatchingFindingRef_136() {
	loop := newLoop()
	rec := newRecordingGateway(modelgateway.NewStubGateway())
	Expect(rubrics.RegisterTools(loop, rec)).To(Succeed())

	content := sampleFileContent(30)
	args, err := json.Marshal(map[string]any{
		"items": []any{
			packItemArgs("ref-a", "a.go", 5, content),
			packItemArgs("ref-b", "b.go", 10, content),
			packItemArgs("ref-c", "c.go", 15, content),
		},
	})
	Expect(err).NotTo(HaveOccurred())

	raw, err := loop.Call(context.Background(), agentloop.CallSourceHandler,
		rubrics.IDHiddenMutationContextualization, args)
	Expect(err).NotTo(HaveOccurred())

	pack := decodePackResults(raw)
	Expect(pack.Results).To(HaveLen(3))

	refs := make([]string, 0, 3)
	for _, r := range pack.Results {
		Expect(r.FindingRef).NotTo(BeEmpty())
		refs = append(refs, r.FindingRef)
		Expect(r.Diagnostic).To(BeNil(), "finding_ref=%s", r.FindingRef)
		Expect(r.HasJudgment()).To(BeTrue(), "finding_ref=%s", r.FindingRef)
		Expect(r.RubricID).To(Equal(rubrics.IDHiddenMutationContextualization))
		Expect(r.RubricVersion).To(Equal(rubrics.Version1))
		Expect(r.ModelIdentity).NotTo(BeNil())
		Expect(*r.ModelIdentity).To(Equal(modelgateway.LogicalModelStub))

		var body map[string]any
		Expect(json.Unmarshal(r.Judgment, &body)).To(Succeed())
		Expect(body["judgment"]).To(BeElementOf("concern", "acceptable", "unclear"))
		Expect(body["confidence"]).To(BeElementOf("high", "medium", "low"))
		Expect(body["rationale"]).NotTo(BeEmpty())
	}
	Expect(refs).To(ConsistOf("ref-a", "ref-b", "ref-c"))

	// Gateway must receive batch OutputSchema (items envelope), not singular v1 only.
	judgeReqs := rec.requests()
	Expect(judgeReqs).To(HaveLen(1))
	Expect(judgeReqs[0].OutputSchema).NotTo(BeEmpty())
	var sch map[string]any
	Expect(json.Unmarshal(judgeReqs[0].OutputSchema, &sch)).To(Succeed())
	props, ok := sch["properties"].(map[string]any)
	Expect(ok).To(BeTrue())
	_, hasItems := props["items"]
	Expect(hasItems).To(BeTrue(), "pack Judge must use batch envelope OutputSchema with items")
	Expect(canonicalJSON(judgeReqs[0].OutputSchema)).To(Equal(
		canonicalJSON(rubrics.HiddenMutationBatchOutputSchema()),
	))
}

func body_batchAcceptanceTest_returnsTwoValidJudgmentsAndOneDiagnosticForTheMi_194() {
	loop := newLoop()
	// Valid items for ref-a and ref-c only — ref-b missing (partial pack success).
	partial := json.RawMessage(`{
					"items": [
						{
							"finding_ref": "ref-a",
							"judgment": "acceptable",
							"rationale": "ok a",
							"confidence": "high",
							"suggested_focus": null
						},
						{
							"finding_ref": "ref-c",
							"judgment": "concern",
							"rationale": "ok c",
							"confidence": "medium",
							"suggested_focus": "review mutation"
						}
					]
				}`)
	gw := &fixedBatchGateway{judgment: partial}
	Expect(rubrics.RegisterTools(loop, gw)).To(Succeed())

	content := sampleFileContent(20)
	args, err := json.Marshal(map[string]any{
		"items": []any{
			packItemArgs("ref-a", "a.go", 1, content),
			packItemArgs("ref-b", "b.go", 2, content),
			packItemArgs("ref-c", "c.go", 3, content),
		},
	})
	Expect(err).NotTo(HaveOccurred())

	raw, err := loop.Call(context.Background(), agentloop.CallSourceHandler,
		rubrics.IDHiddenMutationContextualization, args)
	Expect(err).NotTo(HaveOccurred())

	pack := decodePackResults(raw)
	Expect(pack.Results).To(HaveLen(3))

	byRef := map[string]rubrics.ToolResult{}
	for _, r := range pack.Results {
		byRef[r.FindingRef] = r
	}
	Expect(byRef).To(HaveKey("ref-a"))
	Expect(byRef).To(HaveKey("ref-b"))
	Expect(byRef).To(HaveKey("ref-c"))

	Expect(byRef["ref-a"].HasJudgment()).To(BeTrue())
	Expect(byRef["ref-a"].Diagnostic).To(BeNil())
	Expect(byRef["ref-c"].HasJudgment()).To(BeTrue())
	Expect(byRef["ref-c"].Diagnostic).To(BeNil())

	Expect(byRef["ref-b"].HasJudgment()).To(BeFalse())
	Expect(byRef["ref-b"].Diagnostic).NotTo(BeNil())
	Expect(byRef["ref-b"].Diagnostic.Message).NotTo(BeEmpty())
	Expect(byRef["ref-b"].Diagnostic.Scope).To(ContainSubstring(rubrics.IDHiddenMutationContextualization))
}

func body_batchAcceptanceTest_returnsCannedBatchJSONThatIncludesThoseFindingRe_404() {
	gw := modelgateway.NewStubGateway()
	schema := rubrics.HiddenMutationBatchOutputSchema()
	resp, err := gw.Judge(context.Background(), modelgateway.JudgmentRequest{
		RubricID:      rubrics.IDHiddenMutationContextualization,
		RubricVersion: rubrics.Version1,
		Messages: []modelgateway.Message{
			{Role: "user", Content: "finding_ref: pack-1\nfinding_ref: pack-2\n"},
		},
		OutputSchema: schema,
	})
	Expect(err).NotTo(HaveOccurred())

	var body struct {
		Items []struct {
			FindingRef string `json:"finding_ref"`
			Judgment   string `json:"judgment"`
			Rationale  string `json:"rationale"`
			Confidence string `json:"confidence"`
		} `json:"items"`
	}
	Expect(json.Unmarshal(resp.JudgmentJSON, &body)).To(Succeed())
	Expect(body.Items).To(HaveLen(2))
	Expect(body.Items[0].FindingRef).To(Equal("pack-1"))
	Expect(body.Items[1].FindingRef).To(Equal("pack-2"))
	for _, it := range body.Items {
		Expect(it.Judgment).To(BeElementOf("concern", "acceptable", "unclear"))
		Expect(it.Confidence).To(BeElementOf("high", "medium", "low"))
		Expect(it.Rationale).NotTo(BeEmpty())
	}
}

func body_batchAcceptanceTest_returnsADiagnosticForTheBadRefOnlyAndJudgmentsFo_569() {
	loop := newLoop()
	mixed := json.RawMessage(`{
					"items": [
						{
							"finding_ref": "ref-a",
							"judgment": "acceptable",
							"rationale": "ok a",
							"confidence": "high",
							"suggested_focus": null
						},
						{
							"finding_ref": "ref-b",
							"judgment": "not-a-valid-enum",
							"rationale": "bad b",
							"confidence": "high",
							"suggested_focus": null
						},
						{
							"finding_ref": "ref-c",
							"judgment": "concern",
							"rationale": "ok c",
							"confidence": "medium",
							"suggested_focus": "focus c"
						}
					]
				}`)
	gw := &fixedBatchGateway{judgment: mixed}
	Expect(rubrics.RegisterTools(loop, gw)).To(Succeed())

	content := sampleFileContent(20)
	args, err := json.Marshal(map[string]any{
		"items": []any{
			packItemArgs("ref-a", "a.go", 1, content),
			packItemArgs("ref-b", "b.go", 2, content),
			packItemArgs("ref-c", "c.go", 3, content),
		},
	})
	Expect(err).NotTo(HaveOccurred())

	raw, err := loop.Call(context.Background(), agentloop.CallSourceHandler,
		rubrics.IDHiddenMutationContextualization, args)
	Expect(err).NotTo(HaveOccurred())

	pack := decodePackResults(raw)
	Expect(pack.Results).To(HaveLen(3))
	byRef := map[string]rubrics.ToolResult{}
	for _, r := range pack.Results {
		byRef[r.FindingRef] = r
	}

	Expect(byRef["ref-a"].HasJudgment()).To(BeTrue())
	Expect(byRef["ref-a"].Diagnostic).To(BeNil())
	Expect(byRef["ref-c"].HasJudgment()).To(BeTrue())
	Expect(byRef["ref-c"].Diagnostic).To(BeNil())

	Expect(byRef["ref-b"].HasJudgment()).To(BeFalse())
	Expect(byRef["ref-b"].Diagnostic).NotTo(BeNil())
	Expect(byRef["ref-b"].Diagnostic.Message).To(ContainSubstring("enum"))
}

func body_batchAcceptanceTest_returnsADiagnosticForTheBadRefOnlyAndJudgmentsFo_632() {
	loop := newLoop()
	mixed := json.RawMessage(`{
					"items": [
						{
							"finding_ref": "ref-a",
							"judgment": "acceptable",
							"rationale": "ok a",
							"confidence": "high",
							"suggested_focus": null
						},
						{
							"finding_ref": "ref-b",
							"judgment": "concern",
							"rationale": "bad focus type",
							"confidence": "high",
							"suggested_focus": 42
						},
						{
							"finding_ref": "ref-c",
							"judgment": "unclear",
							"rationale": "ok c",
							"confidence": "low",
							"suggested_focus": null
						}
					]
				}`)
	gw := &fixedBatchGateway{judgment: mixed}
	Expect(rubrics.RegisterTools(loop, gw)).To(Succeed())

	content := sampleFileContent(20)
	args, err := json.Marshal(map[string]any{
		"items": []any{
			packItemArgs("ref-a", "a.go", 1, content),
			packItemArgs("ref-b", "b.go", 2, content),
			packItemArgs("ref-c", "c.go", 3, content),
		},
	})
	Expect(err).NotTo(HaveOccurred())

	raw, err := loop.Call(context.Background(), agentloop.CallSourceHandler,
		rubrics.IDHiddenMutationContextualization, args)
	Expect(err).NotTo(HaveOccurred())

	pack := decodePackResults(raw)
	byRef := map[string]rubrics.ToolResult{}
	for _, r := range pack.Results {
		byRef[r.FindingRef] = r
	}

	Expect(byRef["ref-a"].HasJudgment()).To(BeTrue())
	Expect(byRef["ref-a"].Diagnostic).To(BeNil())
	Expect(byRef["ref-c"].HasJudgment()).To(BeTrue())
	Expect(byRef["ref-c"].Diagnostic).To(BeNil())

	Expect(byRef["ref-b"].HasJudgment()).To(BeFalse())
	Expect(byRef["ref-b"].Diagnostic).NotTo(BeNil())
	Expect(byRef["ref-b"].Diagnostic.Message).To(Or(
		ContainSubstring("suggested_focus"),
		ContainSubstring("schema validation"),
	))
}
