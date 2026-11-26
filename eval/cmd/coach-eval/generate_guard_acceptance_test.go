package main_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func (c programCell) replacing(flag, value string) []string {
	args := c.flags()
	index := slices.Index(args, "--"+flag)
	Expect(index).To(BeNumerically(">=", 0), flag)
	args[index+1] = value
	return args
}

func (c programCell) without(flag string) []string {
	args := c.flags()
	index := slices.Index(args, "--"+flag)
	Expect(index).To(BeNumerically(">=", 0), flag)
	return slices.Delete(args, index, index+2)
}

var dimensionFlags = []string{"layers", "packages", "entry", "depth", "field", "direction", "trap"}

var _ = Describe("coach-eval generate input guards", func() {
	var cacheHome string

	BeforeEach(func() {
		cacheHome = GinkgoT().TempDir()
	})

	DescribeTable("rejects each dimension that alone is outside the cell space (AC-9)",
		func(flag, value string) {
			outcome := runGenerate(cacheHome, nil, baseCell.replacing(flag, value)...)

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stdout).To(BeEmpty())
			Expect(outcome.stderr).To(ContainSubstring(flag))
		},
		Entry("layers below 3", "layers", "2"),
		Entry("packages above 4", "packages", "5"),
		Entry("packages below 2", "packages", "1"),
		Entry("entry that is not a kind", "entry", "lambda"),
		Entry("depth above 5", "depth", "6"),
		Entry("depth below 2", "depth", "1"),
		Entry("field that is not a type", "field", "tuple"),
		Entry("direction that is not a direction", "direction", "sideways"),
		Entry("trap that is not a trap", "trap", "other"),
	)

	DescribeTable("names the one dimension that is missing when the other six are given (AC-9)",
		func(flag string) {
			outcome := runGenerate(cacheHome, nil, baseCell.without(flag)...)

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stdout).To(BeEmpty())
			Expect(outcome.stderr).To(ContainSubstring("missing --" + flag))
		},
		Entry("layers", "layers"),
		Entry("packages", "packages"),
		Entry("entry", "entry"),
		Entry("depth", "depth"),
		Entry("field", "field"),
		Entry("direction", "direction"),
		Entry("trap", "trap"),
	)

	It("rejects a panel that is neither tuning nor held_out (AC-9)", func() {
		outcome := runGenerate(cacheHome, nil, "--cell", "0", "--panel", "other")

		Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring("panel"))
	})

	It("rejects a cell index given together with a dimension, naming both (AC-6)", func() {
		outcome := runGenerate(cacheHome, nil, "--cell", "100", "--layers", "5")

		Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
		Expect(outcome.stdout).To(BeEmpty())
		Expect(outcome.stderr).To(ContainSubstring("--cell"))
		Expect(outcome.stderr).To(ContainSubstring("--layers"))
	})
})

var _ = Describe("coach-eval generate keeps the oracle and the cache honest", func() {
	var cacheHome string

	BeforeEach(func() {
		cacheHome = GinkgoT().TempDir()
	})

	When("the held-out panel is generated with EVAL_ORACLE_DIR set", func() {
		It("writes the oracle under that directory and nothing of it beside the program (AC-10)", func() {
			oracleHome := GinkgoT().TempDir()

			outcome := runGenerate(cacheHome, []string{"EVAL_ORACLE_DIR=" + oracleHome}, "--cell", "1", "--seed", "4", "--panel", "held_out")
			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			program := decodeGenerated(outcome.stdout)

			Expect(program.OracleDir).To(HavePrefix(oracleHome))
			Expect(readTree(program.OracleDir)).To(HaveKey(MatchRegexp(`^tests/tests/feature/.*_test\.go$`)))
			Expect(readTree(program.OracleDir)).To(HaveKey(MatchRegexp(`^solution/.*\.go$`)))
			beside := readTree(filepath.Dir(program.Path))
			for path := range beside {
				Expect(path).NotTo(MatchRegexp(`feature_test\.go|^oracle/`), "the cache directory beside the program holds oracle material")
			}
		})
	})

	When("a cached oracle no longer matches the id its manifest records", func() {
		It("reports a miss and restores the oracle (AC-7)", func() {
			args := append(baseCell.flags(), "--seed", "6")
			first := generateOne(cacheHome, args...)
			original := readTree(first.OracleDir)
			var solutionPath string
			for path := range original {
				if strings.HasPrefix(path, "solution/") {
					solutionPath = path
				}
			}
			Expect(solutionPath).NotTo(BeEmpty())
			Expect(os.WriteFile(filepath.Join(first.OracleDir, solutionPath), []byte("package tampered\n"), 0o644)).To(Succeed())

			repaired := generateOne(cacheHome, args...)

			Expect(repaired.Cache).To(Equal("miss"))
			Expect(readTree(repaired.OracleDir)).To(Equal(original))
		})
	})

	When("--out names a directory that already holds an older copy of the program", func() {
		It("replaces the copy, so a leftover file never reaches the agent (AC-8)", func() {
			out := GinkgoT().TempDir()
			args := append(baseCell.flags(), "--seed", "5", "--out", out)
			first := generateOne(cacheHome, args...)
			leftover := filepath.Join(out, first.ProgramID, "leftover.txt")
			Expect(os.WriteFile(leftover, []byte("stale\n"), 0o644)).To(Succeed())

			generateOne(cacheHome, args...)

			Expect(readTree(filepath.Join(out, first.ProgramID))).To(Equal(readTree(first.Path)))
		})
	})
})
