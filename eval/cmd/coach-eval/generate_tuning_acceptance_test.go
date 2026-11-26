package main_test

import (
	"encoding/json"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach-eval generate --tuning", func() {
	It("emits the twelve fixed tuning cells, one valid program each, and the same programs again from a clean cache (AC-12)", func() {
		first := runGenerate(GinkgoT().TempDir(), nil, "--tuning")
		Expect(first.exitCode).To(Equal(0), first.stderr)
		second := runGenerate(GinkgoT().TempDir(), nil, "--tuning")
		Expect(second.exitCode).To(Equal(0), second.stderr)

		firstPrograms := decodeAllGenerated(first.stdout)
		secondPrograms := decodeAllGenerated(second.stdout)

		Expect(firstPrograms).To(HaveLen(12))
		cells := map[string]bool{}
		for index, program := range firstPrograms {
			encoded, _ := json.Marshal(program.Cell)
			cells[string(encoded)] = true
			Expect(program.ProgramID).To(Equal(treeID(readTree(program.Path))), "program %d", index)
			Expect(secondPrograms[index].ProgramID).To(Equal(program.ProgramID), "program %d regenerates identically", index)
			Expect(secondPrograms[index].Cell).To(Equal(program.Cell))
		}
		Expect(cells).To(HaveLen(12), "twelve distinct cells")
	})

	DescribeTable("is refused beside a cell or another panel (AC-12)",
		func(mentions string, args ...string) {
			outcome := runGenerate(GinkgoT().TempDir(), []string{"EVAL_ORACLE_DIR=" + GinkgoT().TempDir()}, args...)

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stdout).To(BeEmpty())
			Expect(outcome.stderr).To(ContainSubstring("--tuning"))
			Expect(outcome.stderr).To(ContainSubstring(mentions))
		},
		Entry("a cell index", "--cell", "--tuning", "--cell", "0"),
		Entry("a dimension", "--layers", "--tuning", "--layers", "3"),
		Entry("the held-out panel", "held_out", "--tuning", "--panel", "held_out"),
	)

	It("keeps every tuning cell out of the held-out panel (AC-13)", func() {
		tuning := runGenerate(GinkgoT().TempDir(), nil, "--tuning")
		Expect(tuning.exitCode).To(Equal(0), tuning.stderr)
		cell := decodeAllGenerated(tuning.stdout)[0].Cell

		outcome := runGenerate(GinkgoT().TempDir(), []string{"EVAL_ORACLE_DIR=" + GinkgoT().TempDir()},
			"--layers", fmt.Sprint(cell.Layers), "--packages", fmt.Sprint(cell.Packages), "--entry", cell.Entry,
			"--depth", fmt.Sprint(cell.Depth), "--field", cell.Field, "--direction", cell.Direction,
			"--trap", cell.Trap, "--panel", "held_out")

		Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
		Expect(outcome.stdout).To(BeEmpty())
		Expect(outcome.stderr).To(ContainSubstring("tuning"))
	})
})
