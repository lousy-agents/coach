package evalharness_test

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
	"github.com/lousy-agents/coach/eval/internal/evalharness"
)

const complexFunction = `package %s

func Tangle(values []int, limit int) int {
	total := 0
	for _, a := range values {
		if a > limit {
			for _, b := range values {
				if b > a {
					for _, c := range values {
						if c > b {
							if a+b+c > limit {
								total += a
							} else if a+b > c {
								total += b
							}
						}
					}
				}
			}
		}
	}
	return total
}
`

func program(cellIndex int) evalgen.Program {
	cell, err := evalgen.CellFromIndex(cellIndex)
	Expect(err).NotTo(HaveOccurred())
	generated, err := evalgen.Generate(cell, 1)
	Expect(err).NotTo(HaveOccurred())
	return generated
}

func writeBase(generated evalgen.Program) string {
	directory := GinkgoT().TempDir()
	Expect(evalgen.WriteTree(directory, generated.Tree)).To(Succeed())
	return directory
}

var _ = Describe("the graded detectors over a generated base", func() {
	It("report nothing on an unmutated program (AC-17)", func() {
		Expect(evalharness.GradeBase(writeBase(program(0)))).To(Succeed())
	})

	It("reject a base with an import across the forbidden pair, naming the rule (AC-17)", func() {
		generated := program(0)
		directory := writeBase(generated)
		first, last := firstAndLastLayerDirectories(directory)
		leak := "package " + filepath.Base(first) + "\n\nimport _ \"" + moduleOf(generated) + "/" + filepath.ToSlash(last) + "\"\n"
		Expect(os.WriteFile(filepath.Join(directory, first, "leak.go"), []byte(leak), 0o644)).To(Succeed())

		err := evalharness.GradeBase(directory)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("architecture.layer_violation"))
		Expect(rejectionStep(err)).To(Equal(evalgen.StepLayerGraph))
	})

	It("reject a base with a function at or over the cognitive-complexity threshold, naming the rule (AC-17)", func() {
		directory := writeBase(program(0))
		first, _ := firstAndLastLayerDirectories(directory)
		source := strings.Replace(complexFunction, "%s", filepath.Base(first), 1)
		Expect(os.WriteFile(filepath.Join(directory, first, "tangle.go"), []byte(source), 0o644)).To(Succeed())

		err := evalharness.GradeBase(directory)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("complexity.cognitive_complexity"))
		Expect(rejectionStep(err)).To(Equal(evalgen.StepGocognit))
	})
})

func rejectionStep(err error) string {
	var stepError *evalgen.StepError
	Expect(errors.As(err, &stepError)).To(BeTrue(), "the grader rejects with a step, not a plain error: %v", err)
	return stepError.Step
}

func moduleOf(generated evalgen.Program) string {
	first, _, _ := strings.Cut(strings.TrimPrefix(string(generated.Tree["go.mod"]), "module "), "\n")
	return first
}

func firstAndLastLayerDirectories(root string) (first, last string) {
	layers := layerDirectories(root)
	return layers[0], layers[len(layers)-1]
}

func layerDirectories(root string) []string {
	policy, err := os.ReadFile(filepath.Join(root, ".lousy-agents", "coach.json"))
	Expect(err).NotTo(HaveOccurred())
	var parsed struct {
		Layers []struct {
			Prefixes []string `json:"prefixes"`
		} `json:"layers"`
	}
	Expect(json.Unmarshal(policy, &parsed)).To(Succeed())
	directories := make([]string, 0, len(parsed.Layers))
	for _, layer := range parsed.Layers {
		packages, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(layer.Prefixes[0])))
		Expect(err).NotTo(HaveOccurred())
		directories = append(directories, layer.Prefixes[0]+"/"+packages[0].Name())
	}
	return directories
}
