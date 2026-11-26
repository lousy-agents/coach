package main_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach-eval generate --tuning --language typescript (781 tuning-cell row)", Label("typescript"), func() {
	It("emits the same twelve tuning cells as Go, in the same order, one valid TypeScript program each", func() {
		outcome := runGenerate(GinkgoT().TempDir(), nil, "--tuning", "--language", "typescript")
		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)

		programs := decodeAllGenerated(outcome.stdout)

		Expect(programs).To(HaveLen(12))
		tuned := make([]generatedCell, 0, len(programs))
		for index, program := range programs {
			tuned = append(tuned, program.Cell)
			tree := readTree(program.Path)
			Expect(tree).To(HaveKey("tsconfig.json"), "program %d is TypeScript", index)
			Expect(tree).NotTo(HaveKey("go.mod"), "program %d", index)
			Expect(program.ProgramID).To(Equal(treeID(tree)), "program %d", index)
		}
		Expect(tuned).To(Equal(pinnedTuningCells), "the twelve Go cells, in this order")
	})

	It("regenerates the same programs from a clean cache, and none of them is the Go program of its cell", func() {
		first := decodeAllGenerated(runGenerate(GinkgoT().TempDir(), nil, "--tuning", "--language", "typescript").stdout)
		second := decodeAllGenerated(runGenerate(GinkgoT().TempDir(), nil, "--tuning", "--language", "typescript").stdout)
		goPrograms := decodeAllGenerated(runGenerate(GinkgoT().TempDir(), nil, "--tuning").stdout)

		Expect(first).To(HaveLen(12))
		for index, program := range first {
			Expect(second[index].ProgramID).To(Equal(program.ProgramID), "program %d regenerates identically", index)
			Expect(goPrograms[index].ProgramID).NotTo(Equal(program.ProgramID), "program %d", index)
		}
	})

	DescribeTable("is still refused beside a cell, a dimension or the held-out panel",
		func(mentions string, args ...string) {
			outcome := runGenerate(GinkgoT().TempDir(), []string{"EVAL_ORACLE_DIR=" + GinkgoT().TempDir()}, append([]string{"--tuning", "--language", "typescript"}, args...)...)

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stdout).To(BeEmpty())
			Expect(outcome.stderr).To(ContainSubstring("--tuning"))
			Expect(outcome.stderr).To(ContainSubstring(mentions))
		},
		Entry("a cell index", "--cell", "--cell", "0"),
		Entry("a dimension", "--layers", "--layers", "3"),
		Entry("the held-out panel", "held_out", "--panel", "held_out"),
	)

	It("keeps a tuning cell out of the TypeScript held-out panel as it does for Go", func() {
		cell := pinnedTuningCells[0]

		outcome := runGenerate(GinkgoT().TempDir(), []string{"EVAL_ORACLE_DIR=" + GinkgoT().TempDir()},
			"--language", "typescript", "--layers", fmt.Sprint(cell.Layers), "--packages", fmt.Sprint(cell.Packages), "--entry", cell.Entry,
			"--depth", fmt.Sprint(cell.Depth), "--field", cell.Field, "--direction", cell.Direction, "--trap", cell.Trap, "--panel", "held_out")

		Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
		Expect(outcome.stdout).To(BeEmpty())
		Expect(outcome.stderr).To(ContainSubstring("tuning"))
	})
})
