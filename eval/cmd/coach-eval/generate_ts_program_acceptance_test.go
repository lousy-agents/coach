package main_test

import (
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("a generated TypeScript program", func() {
	for _, cell := range programCells {
		It("type-checks with the stub, fails only its feature tests, and passes them with the oracle ("+strings.Join(cell.flags(), " ")+") (781 AC-2, done-when)", func() {
			program := generateOne(GinkgoT().TempDir(), append(typeScriptCellFlags(cell), "--seed", "9")...)
			tree := readTree(program.Path)
			work := GinkgoT().TempDir()
			copyTree(tree, work)
			visible := filesMatching(tree, `\.test\.ts$`)
			Expect(visible).NotTo(BeEmpty())

			stub := tscIn(work, "-p", ".")
			Expect(stub.exitCode).To(Equal(0), "the stub type-checks: "+stub.stdout+stub.stderr)
			visibleRun := nodeTest(work, visible...)
			Expect(visibleRun.exitCode).To(Equal(0), "the visible PASS_TO_PASS tests pass on the stub: "+visibleRun.stdout+visibleRun.stderr)

			hidden := readTree(filepath.Join(program.OracleDir, "tests"))
			Expect(hidden).To(HaveLen(1))
			copyTree(hidden, work)
			stillStubs := tscIn(work, "-p", ".")
			Expect(stillStubs.exitCode).To(Equal(0), "the hidden tests sit outside the tsconfig, so the stub still type-checks: "+stillStubs.stdout)
			featureRun := nodeTest(work, "tests/feature/feature.test.ts")
			Expect(featureRun.exitCode).NotTo(Equal(0), "the FAIL_TO_PASS tests fail on the stub")
			Expect(nodeTest(work, visible...).exitCode).To(Equal(0), "no visible test fails once the hidden ones are present")

			solution := readTree(filepath.Join(program.OracleDir, "solution"))
			Expect(solution).NotTo(BeEmpty())
			copyTree(solution, work)
			Expect(tscIn(work, "-p", ".").exitCode).To(Equal(0))
			oracle := nodeTest(work, append(visible, "tests/feature/feature.test.ts")...)
			Expect(oracle.exitCode).To(Equal(0), "the clean oracle passes every test: "+oracle.stdout+oracle.stderr)
		})
	}

	It("keeps the oracle out of the tree and the oracle directory out of the repository", func() {
		cacheHome := GinkgoT().TempDir()
		program := generateOne(cacheHome, append(typeScriptCellFlags(programCells[0]), "--seed", "9")...)

		Expect(program.OracleDir).To(HavePrefix(cacheHome))
		Expect(readTree(program.Path)).NotTo(HaveKey(MatchRegexp(`^(tests|solution)/`)))
	})
})

var _ = Describe("a generated TypeScript program of the held_out panel", func() {
	It("keeps its hidden tests and its oracle under EVAL_ORACLE_DIR, and out of the program's cache directory", func() {
		cacheHome, oracleHome := GinkgoT().TempDir(), GinkgoT().TempDir()
		outcome := runGenerate(cacheHome, []string{"EVAL_ORACLE_DIR=" + oracleHome},
			append(typeScriptCellFlags(programCells[2]), "--seed", "9", "--panel", "held_out")...)
		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		program := decodeGenerated(outcome.stdout)

		Expect(program.OracleDir).To(HavePrefix(oracleHome))
		Expect(readTree(program.OracleDir)).To(HaveKey(MatchRegexp(`^tests/.*feature\.test\.ts$`)))
		Expect(readTree(filepath.Dir(program.Path))).NotTo(HaveKey(MatchRegexp(`feature\.test\.ts$`)))
	})
})
