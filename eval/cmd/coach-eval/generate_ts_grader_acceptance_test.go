package main_test

import (
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evaldetect"
	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// expectGraderAccepts runs the independent layer grader over a tree and requires neither a finding nor a refusal.
func expectGraderAccepts(files map[string][]byte, label string) {
	root := GinkgoT().TempDir()
	Expect(evalgen.WriteTree(root, files)).To(Succeed())
	policy, err := evaldetect.LoadPolicy(filepath.Join(root, ".lousy-agents", "coach.json"))
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), label)

	findings, err := evaldetect.TSLayerViolations(root, policy)

	ExpectWithOffset(1, err).NotTo(HaveOccurred(), label+": the grader refused the program")
	ExpectWithOffset(1, findings).To(BeEmpty(), label+": the base program holds a forbidden edge")
}

func overlaid(tree, solution map[string][]byte) map[string][]byte {
	merged := map[string][]byte{}
	for path, content := range tree {
		merged[path] = content
	}
	for path, content := range solution {
		merged[path] = content
	}
	return merged
}

var _ = Describe("the TypeScript layer grader and every generated TypeScript program", func() {
	It("accepts the stub and the oracle of every cell with no refusal and no finding (781 constraint)", func() {
		for index := 0; index < evalgen.CellCount; index++ {
			cell, err := evalgen.CellFromIndex(index)
			Expect(err).NotTo(HaveOccurred())
			program, err := evalgen.GenerateTypeScript(cell, 0)
			Expect(err).NotTo(HaveOccurred())

			expectGraderAccepts(program.Tree, fmt.Sprintf("cell %d stub", index))
			expectGraderAccepts(overlaid(program.Tree, program.Solution), fmt.Sprintf("cell %d oracle", index))
		}
	})
})
