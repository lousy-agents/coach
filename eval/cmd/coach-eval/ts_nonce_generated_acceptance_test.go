package main_test

import (
	"fmt"
	"os/exec"
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
	"github.com/lousy-agents/coach/eval/internal/evalgrade"
)

const generatedRunNonce = "GEN-NONCE-4c19b7"

func (c programCell) cell() evalgen.Cell {
	return evalgen.Cell{Layers: c.layers, Packages: c.packages, Entry: c.entry, Depth: c.depth, Field: c.field, Direction: c.direction, Trap: c.trap}
}

func expectedTestIDs(program evalgen.Program) []string {
	ids := []string{}
	for _, visible := range program.Manifest.Template.VisibleTests {
		for _, name := range visible.TestIDs {
			ids = append(ids, visible.Path+"::"+name)
		}
	}
	sort.Strings(ids)
	return ids
}

func runThroughNonceChannel(files map[string][]byte, paths []string) evalgrade.TypeScriptRun {
	root := GinkgoT().TempDir()
	Expect(evalgen.WriteTree(root, files)).To(Succeed())
	node, err := exec.LookPath("node")
	Expect(err).NotTo(HaveOccurred(), "node 24 must be on PATH")

	run, err := evalgrade.RunTypeScriptTests(node, root, paths, generatedRunNonce)

	Expect(err).NotTo(HaveOccurred())
	return run
}

func hiddenPaths(program evalgen.Program) []string {
	paths := []string{}
	for path := range program.Tests {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

var _ = Describe("a generated TypeScript program under the nonce channel (781 AC-3)", func() {
	It("gives every visible test of every cell exactly one passing record under the ID the manifest lists", func() {
		for index, shape := range programCells {
			program, err := evalgen.GenerateTypeScript(shape.cell(), 0)
			Expect(err).NotTo(HaveOccurred())
			visible := []string{}
			for _, test := range program.Manifest.Template.VisibleTests {
				visible = append(visible, test.Path)
			}

			run := runThroughNonceChannel(program.Tree, visible)
			verdicts := evalgrade.Verify(run.Records, generatedRunNonce, expectedTestIDs(program))

			Expect(verdicts.AllPass()).To(BeTrue(), fmt.Sprintf("cell %d: %+v", index, verdicts))
			Expect(verdicts.Forged).To(BeZero())
			Expect(verdicts.Unexpected).To(BeEmpty(), fmt.Sprintf("cell %d reports a test the manifest does not list", index))
		}
	})

	It("fails every hidden test on the stub and passes every one on the oracle, for every cell", func() {
		for index, shape := range programCells {
			program, err := evalgen.GenerateTypeScript(shape.cell(), 0)
			Expect(err).NotTo(HaveOccurred())
			hidden := hiddenPaths(program)
			expected := []string{}
			for _, path := range hidden {
				expected = append(expected, path+"::"+hiddenTestName)
			}

			stub := runThroughNonceChannel(overlaid(program.Tree, program.Tests), hidden)
			oracle := runThroughNonceChannel(overlaid(overlaid(program.Tree, program.Solution), program.Tests), hidden)

			stubVerdicts := evalgrade.Verify(stub.Records, generatedRunNonce, expected)
			oracleVerdicts := evalgrade.Verify(oracle.Records, generatedRunNonce, expected)
			Expect(stubVerdicts.AllPass()).To(BeFalse(), fmt.Sprintf("cell %d: the hidden test passes on the stub", index))
			Expect(stubVerdicts.ByID[expected[0]]).To(Equal(evalgrade.VerdictFail), fmt.Sprintf("cell %d: it fails for a reason, not by going missing", index))
			Expect(oracleVerdicts.AllPass()).To(BeTrue(), fmt.Sprintf("cell %d: %+v", index, oracleVerdicts))
		}
	})
})

// hiddenTestName is the one test every generated hidden file holds.
const hiddenTestName = "the entry exposes the field"
