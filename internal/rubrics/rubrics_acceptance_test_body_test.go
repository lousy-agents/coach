package rubrics_test

import (
	"context"
	"encoding/json"

	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/modelgateway"
	"github.com/lousy-agents/coach/internal/rubrics"
)

func body_rubricsAcceptanceTest_exposesExactlyTheTwoVersionedSeedRubricsDecidedF_33() {
	seed := rubrics.Seed()
	Expect(seed).To(HaveLen(2))

	byID := map[string]rubrics.Definition{}
	for _, def := range seed {
		byID[def.ID] = def
	}

	hidden, ok := byID[rubrics.IDHiddenMutationContextualization]
	Expect(ok).To(BeTrue(), "missing hidden_mutation_contextualization")
	Expect(hidden.Version).To(Equal(rubrics.Version1))
	Expect(hidden.OutputSchema).NotTo(BeEmpty())

	cohesion, ok := byID[rubrics.IDChangeCohesion]
	Expect(ok).To(BeTrue(), "missing change_cohesion")
	Expect(cohesion.Version).To(Equal(rubrics.Version1))
	Expect(cohesion.OutputSchema).NotTo(BeEmpty())
}

func body_rubricsAcceptanceTest_validatesSuccessfullyAndMatchesTheCommittedGolde_57() {
	cases := []struct {
		id         string
		goldenFile string
	}{
		{rubrics.IDHiddenMutationContextualization, "hidden_mutation_contextualization_v1.json"},
		{rubrics.IDChangeCohesion, "change_cohesion_v1.json"},
	}

	gw := modelgateway.NewStubGateway()
	for _, tc := range cases {
		def := seedByID(tc.id)
		resp, err := gw.Judge(context.Background(), modelgateway.JudgmentRequest{
			RubricID:      def.ID,
			RubricVersion: def.Version,
			Messages: []modelgateway.Message{
				{Role: "user", Content: "fixture judgment"},
			},
			OutputSchema: def.OutputSchema,
		})
		Expect(err).NotTo(HaveOccurred(), "schema must accept stub judgment for %s", tc.id)
		Expect(resp.LogicalModelID).To(Equal(modelgateway.LogicalModelStub))

		golden := readGolden(tc.goldenFile)
		Expect(canonicalJSON(resp.JudgmentJSON)).To(Equal(canonicalJSON(golden)),
			"stub judgment for %s must match golden byte-identically (canonical JSON)", tc.id)

		resp2, err := gw.Judge(context.Background(), modelgateway.JudgmentRequest{
			RubricID:      def.ID,
			RubricVersion: def.Version,
			Messages:      []modelgateway.Message{{Role: "user", Content: "round-trip"}},
			OutputSchema:  def.OutputSchema,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(canonicalJSON(resp2.JudgmentJSON)).To(Equal(canonicalJSON(golden)))
	}
}

func body_rubricsAcceptanceTest_usesOnlyTheModelgatewayValidationSubsetObjectReq_97() {
	seed := rubrics.Seed()
	Expect(seed).NotTo(BeEmpty())
	for _, def := range seed {
		var sch map[string]any
		Expect(json.Unmarshal(def.OutputSchema, &sch)).To(Succeed())
		Expect(sch["type"]).To(Equal("object"))

		required, ok := sch["required"].([]any)
		Expect(ok).To(BeTrue())
		Expect(required).To(ConsistOf("judgment", "rationale", "confidence", "suggested_focus"))

		props, ok := sch["properties"].(map[string]any)
		Expect(ok).To(BeTrue())

		for _, key := range []string{"judgment", "rationale", "confidence", "suggested_focus"} {
			_, present := props[key]
			Expect(present).To(BeTrue(), "%s schema missing property %s", def.ID, key)
		}

		sf, ok := props["suggested_focus"].(map[string]any)
		Expect(ok).To(BeTrue())
		Expect(sf["type"]).To(ConsistOf("string", "null"))

		judgmentProp := props["judgment"].(map[string]any)
		Expect(judgmentProp["type"]).To(Equal("string"))
		enumVals, ok := judgmentProp["enum"].([]any)
		Expect(ok).To(BeTrue())
		switch def.ID {
		case rubrics.IDHiddenMutationContextualization:
			Expect(enumVals).To(ConsistOf("concern", "acceptable", "unclear"))
		case rubrics.IDChangeCohesion:
			Expect(enumVals).To(ConsistOf("focused", "diffuse", "unclear"))
		}

		confProp := props["confidence"].(map[string]any)
		Expect(confProp["enum"]).To(ConsistOf("high", "medium", "low"))
	}
}

func body_rubricsAcceptanceTest_attachesOneDeterministicHiddenInputMutationFindi_141() {
	finding := sampleHiddenMutationFinding()
	file := rubrics.FileContext{
		Path:     "pkg/example/service.go",
		Language: "go",
		Content:  "package example\n\nfunc NewService(cfg *Config) *Service { cfg.timeout = 1; return &Service{} }\n",
	}

	msgs := rubrics.AssembleHiddenMutationMessages(rubrics.HiddenMutationEvidence{
		Finding: finding,
		File:    file,
	})
	Expect(msgs).NotTo(BeEmpty())

	joined := ""
	for _, m := range msgs {
		Expect(m.Role).NotTo(BeEmpty())
		Expect(m.Content).NotTo(BeEmpty())
		joined += m.Content
	}
	Expect(joined).To(ContainSubstring("state.hidden_input_mutation"))
	Expect(joined).To(ContainSubstring("hidden_input_mutation"))
	Expect(joined).To(ContainSubstring("pkg/example/service.go"))
	Expect(joined).To(ContainSubstring("cfg.timeout = 1"))
	Expect(joined).To(ContainSubstring("NewService"))
}

func body_rubricsAcceptanceTest_attachesTheFullSetOfDeterministicFindingsAndFile_170() {
	findings := sampleDeterministicFindings()
	files := []rubrics.FileMeta{
		{Path: "pkg/example/service.go", Language: "go"},
		{Path: "pkg/example/client.go", Language: "go"},
	}

	msgs := rubrics.AssembleChangeCohesionMessages(rubrics.ChangeCohesionEvidence{
		Findings: findings,
		Files:    files,
	})
	Expect(msgs).NotTo(BeEmpty())

	joined := ""
	for _, m := range msgs {
		Expect(m.Role).NotTo(BeEmpty())
		Expect(m.Content).NotTo(BeEmpty())
		joined += m.Content
	}
	Expect(joined).To(ContainSubstring("state.hidden_input_mutation"))
	Expect(joined).To(ContainSubstring("constructor.tight_init"))
	Expect(joined).To(ContainSubstring("pkg/example/service.go"))
	Expect(joined).To(ContainSubstring("pkg/example/client.go"))
}

func body_rubricsAcceptanceTest_forwardsThatRubricSOutputSchemaAndEvidenceBearin_499() {

	cases := []struct {
		id         string
		msgs       []modelgateway.Message
		evidence   []string
		goldenFile string
	}{
		{
			id: rubrics.IDHiddenMutationContextualization,
			msgs: rubrics.AssembleHiddenMutationMessages(rubrics.HiddenMutationEvidence{
				Finding: sampleHiddenMutationFinding(),
				File: rubrics.FileContext{
					Path:     "pkg/example/service.go",
					Language: "go",
					Content:  "package example\nfunc NewService(cfg *Config) *Service { cfg.timeout = 1; return &Service{} }\n",
				},
			}),
			evidence: []string{
				"state.hidden_input_mutation",
				"hidden_input_mutation",
				"pkg/example/service.go",
				"cfg.timeout = 1",
				"NewService",
			},
			goldenFile: "hidden_mutation_contextualization_v1.json",
		},
		{
			id: rubrics.IDChangeCohesion,
			msgs: rubrics.AssembleChangeCohesionMessages(rubrics.ChangeCohesionEvidence{
				Findings: sampleDeterministicFindings(),
				Files: []rubrics.FileMeta{
					{Path: "pkg/example/service.go", Language: "go"},
					{Path: "pkg/example/client.go", Language: "go"},
				},
			}),
			evidence: []string{
				"state.hidden_input_mutation",
				"constructor.tight_init",
				"pkg/example/service.go",
				"pkg/example/client.go",
			},
			goldenFile: "change_cohesion_v1.json",
		},
	}

	for _, tc := range cases {
		def := seedByID(tc.id)
		rec := newRecordingGateway(modelgateway.NewStubGateway())
		result, err := rubrics.Run(context.Background(), rec, def, tc.msgs)
		Expect(err).NotTo(HaveOccurred(), "rubric %s", tc.id)
		Expect(result.Diagnostic).To(BeNil(), "rubric %s", tc.id)
		Expect(result.Judgment).NotTo(BeNil(), "rubric %s", tc.id)
		Expect(canonicalJSON(result.Judgment.JudgmentJSON)).To(Equal(
			canonicalJSON(readGolden(tc.goldenFile)),
		))

		judgeReqs := rec.requests()
		Expect(judgeReqs).To(HaveLen(1), "rubric %s", tc.id)
		expectJudgmentRequest(judgeReqs[0], def, tc.evidence...)
	}
}
