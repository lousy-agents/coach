package modelgateway_test

import (
	"context"
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/modelgateway"
)

// Seed rubric output shapes (Task 4 / epic #97) used to assert stub fixture validity.
type hiddenMutationJudgment struct {
	Judgment       string  `json:"judgment"`
	Rationale      string  `json:"rationale"`
	Confidence     string  `json:"confidence"`
	SuggestedFocus *string `json:"suggested_focus"`
}

type changeCohesionJudgment struct {
	Judgment       string  `json:"judgment"`
	Rationale      string  `json:"rationale"`
	Confidence     string  `json:"confidence"`
	SuggestedFocus *string `json:"suggested_focus"`
}

func hiddenMutationSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["judgment", "rationale", "confidence", "suggested_focus"],
		"properties": {
			"judgment": {"type": "string", "enum": ["concern", "acceptable", "unclear"]},
			"rationale": {"type": "string"},
			"confidence": {"type": "string", "enum": ["high", "medium", "low"]},
			"suggested_focus": {"type": ["string", "null"]}
		}
	}`)
}

func changeCohesionSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"required": ["judgment", "rationale", "confidence", "suggested_focus"],
		"properties": {
			"judgment": {"type": "string", "enum": ["focused", "diffuse", "unclear"]},
			"rationale": {"type": "string"},
			"confidence": {"type": "string", "enum": ["high", "medium", "low"]},
			"suggested_focus": {"type": ["string", "null"]}
		}
	}`)
}

var _ = Describe("modelgateway.Gateway", func() {
	Describe("StubGateway", func() {
		When("a rubric-judgment request is made for a seed rubric", func() {
			It("returns a canned schema-valid judgment with model identity for provenance", func() {
				var gw modelgateway.Gateway = modelgateway.NewStubGateway()

				resp, err := gw.Judge(context.Background(), modelgateway.JudgmentRequest{
					RubricID:      "hidden_mutation",
					RubricVersion: "1",
					Messages: []modelgateway.Message{
						{Role: "user", Content: "Judge this change for hidden mutation."},
					},
					OutputSchema: hiddenMutationSchema(),
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(resp.LogicalModelID).NotTo(BeEmpty(), "Story 5 provenance requires logical model id")
				Expect(resp.JudgmentJSON).NotTo(BeEmpty())

				var got hiddenMutationJudgment
				Expect(json.Unmarshal(resp.JudgmentJSON, &got)).To(Succeed())
				Expect(got.Judgment).To(BeElementOf("concern", "acceptable", "unclear"))
				Expect(got.Rationale).NotTo(BeEmpty())
				Expect(got.Confidence).To(BeElementOf("high", "medium", "low"))

				resp2, err := gw.Judge(context.Background(), modelgateway.JudgmentRequest{
					RubricID:      "change_cohesion",
					RubricVersion: "1",
					Messages: []modelgateway.Message{
						{Role: "user", Content: "Judge this change for cohesion."},
					},
					OutputSchema: changeCohesionSchema(),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(resp2.LogicalModelID).NotTo(BeEmpty())

				var got2 changeCohesionJudgment
				Expect(json.Unmarshal(resp2.JudgmentJSON, &got2)).To(Succeed())
				Expect(got2.Judgment).To(BeElementOf("focused", "diffuse", "unclear"))
				Expect(got2.Rationale).NotTo(BeEmpty())
				Expect(got2.Confidence).To(BeElementOf("high", "medium", "low"))
			})
		})
	})

	Describe("typed errors", func() {
		When("callers classify gateway failures", func() {
			It("distinguishes schema validation failure from unavailable/transient with errors.Is and errors.As", func() {
				valErr := modelgateway.NewValidationError("judgment missing required field")
				unavailErr := modelgateway.NewUnavailableError("upstream timeout", context.DeadlineExceeded)

				Expect(errors.Is(valErr, modelgateway.ErrSchemaValidation)).To(BeTrue())
				Expect(errors.Is(valErr, modelgateway.ErrUnavailable)).To(BeFalse())
				Expect(errors.Is(unavailErr, modelgateway.ErrUnavailable)).To(BeTrue())
				Expect(errors.Is(unavailErr, modelgateway.ErrSchemaValidation)).To(BeFalse())

				var asVal *modelgateway.ValidationError
				Expect(errors.As(valErr, &asVal)).To(BeTrue())
				Expect(asVal.Detail).NotTo(BeEmpty())
				Expect(errors.As(unavailErr, &asVal)).To(BeFalse())

				var asUnavail *modelgateway.UnavailableError
				Expect(errors.As(unavailErr, &asUnavail)).To(BeTrue())
				Expect(asUnavail.Detail).NotTo(BeEmpty())
				Expect(errors.As(valErr, &asUnavail)).To(BeFalse())

				// Public Gateway surface: a Judge failure must be classifiable the same way.
				// Force the stub into each failure mode so the seam itself is covered, not only constructors.
				schemaFail := modelgateway.NewStubGateway(modelgateway.StubOptions{
					JudgeErr: modelgateway.NewValidationError("fixture schema mismatch"),
				})
				_, err := schemaFail.Judge(context.Background(), modelgateway.JudgmentRequest{
					RubricID:      "hidden_mutation",
					RubricVersion: "1",
					Messages:      []modelgateway.Message{{Role: "user", Content: "x"}},
					OutputSchema:  hiddenMutationSchema(),
				})
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, modelgateway.ErrSchemaValidation)).To(BeTrue())
				Expect(errors.Is(err, modelgateway.ErrUnavailable)).To(BeFalse())

				down := modelgateway.NewStubGateway(modelgateway.StubOptions{
					JudgeErr: modelgateway.NewUnavailableError("connection refused", errors.New("dial tcp")),
				})
				_, err = down.Judge(context.Background(), modelgateway.JudgmentRequest{
					RubricID:      "hidden_mutation",
					RubricVersion: "1",
					Messages:      []modelgateway.Message{{Role: "user", Content: "x"}},
					OutputSchema:  hiddenMutationSchema(),
				})
				Expect(err).To(HaveOccurred())
				Expect(errors.Is(err, modelgateway.ErrUnavailable)).To(BeTrue())
				Expect(errors.Is(err, modelgateway.ErrSchemaValidation)).To(BeFalse())
			})
		})
	})
})
