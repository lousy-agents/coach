package modelgateway_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/internal/modelgateway"
)

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
			It("returns a canned schema-valid judgment with stub model identity for provenance", func() {
				var gw modelgateway.Gateway = modelgateway.NewStubGateway()

				resp, err := gw.Judge(context.Background(), modelgateway.JudgmentRequest{
					RubricID:      "hidden_mutation_contextualization",
					RubricVersion: "1",
					Messages: []modelgateway.Message{
						{Role: "user", Content: "Judge this change for hidden mutation."},
					},
					OutputSchema: hiddenMutationSchema(),
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(resp.LogicalModelID).To(Equal(modelgateway.LogicalModelStub))
				Expect(resp.JudgmentJSON).NotTo(BeEmpty())

				var got hiddenMutationJudgment
				Expect(json.Unmarshal(resp.JudgmentJSON, &got)).To(Succeed())
				Expect(got.Judgment).To(BeElementOf("concern", "acceptable", "unclear"))
				Expect(got.Rationale).NotTo(BeEmpty())
				Expect(got.Confidence).To(BeElementOf("high", "medium", "low"))
				Expect(got.SuggestedFocus).To(BeNil())

				resp2, err := gw.Judge(context.Background(), modelgateway.JudgmentRequest{
					RubricID:      "change_cohesion",
					RubricVersion: "1",
					Messages: []modelgateway.Message{
						{Role: "user", Content: "Judge this change for cohesion."},
					},
					OutputSchema: changeCohesionSchema(),
				})
				Expect(err).NotTo(HaveOccurred())
				Expect(resp2.LogicalModelID).To(Equal(modelgateway.LogicalModelStub))

				var got2 changeCohesionJudgment
				Expect(json.Unmarshal(resp2.JudgmentJSON, &got2)).To(Succeed())
				Expect(got2.Judgment).To(BeElementOf("focused", "diffuse", "unclear"))
				Expect(got2.Rationale).NotTo(BeEmpty())
				Expect(got2.Confidence).To(BeElementOf("high", "medium", "low"))
				Expect(got2.SuggestedFocus).To(BeNil())
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

				// Cover Judge classification, not only error constructors.
				schemaFail := modelgateway.NewStubGateway(modelgateway.StubOptions{
					JudgeErr: modelgateway.NewValidationError("fixture schema mismatch"),
				})
				_, err := schemaFail.Judge(context.Background(), modelgateway.JudgmentRequest{
					RubricID:      "hidden_mutation_contextualization",
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
					RubricID:      "hidden_mutation_contextualization",
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

	Describe("package surface", func() {
		It("keeps production identifiers free of provider-specific names", func() {
			dir, err := os.Getwd()
			Expect(err).NotTo(HaveOccurred())

			entries, err := os.ReadDir(dir)
			Expect(err).NotTo(HaveOccurred())

			forbidden := []string{"llamacpp", "llama.cpp", "sglang"}
			for _, e := range entries {
				name := e.Name()
				if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
					continue
				}
				lowerName := strings.ToLower(name)
				for _, f := range forbidden {
					Expect(lowerName).NotTo(ContainSubstring(f), "production filename %s", name)
				}
				raw, readErr := os.ReadFile(filepath.Join(dir, name))
				Expect(readErr).NotTo(HaveOccurred())
				lower := strings.ToLower(string(raw))
				for _, f := range forbidden {
					Expect(lower).NotTo(ContainSubstring(f), "production file %s", name)
				}
			}
		})

		It("is the only non-test package that owns the chat-completions wire path", func() {
			root := findModuleRoot()
			Expect(root).NotTo(BeEmpty())

			var offenders []string
			err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if d.IsDir() {
					base := d.Name()
					if base == ".git" || base == "node_modules" || base == "vendor" || base == "dist" || base == "dist-test" {
						return filepath.SkipDir
					}
					return nil
				}
				if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
					return nil
				}
				rel, relErr := filepath.Rel(root, path)
				Expect(relErr).NotTo(HaveOccurred())
				if strings.HasPrefix(rel, "internal"+string(filepath.Separator)+"modelgateway"+string(filepath.Separator)) {
					return nil
				}
				raw, readErr := os.ReadFile(path)
				if readErr != nil {
					return readErr
				}
				if strings.Contains(string(raw), "/v1/chat/completions") {
					offenders = append(offenders, rel)
				}
				return nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(offenders).To(BeEmpty(), "chat-completions path must stay inside internal/modelgateway: %v", offenders)
		})
	})
})

func findModuleRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}
