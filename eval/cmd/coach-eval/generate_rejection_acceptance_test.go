package main_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const faultVariable = "COACH_EVAL_FAULT"

func buildCoachEvalWithFaultSeam() string {
	directory, err := os.MkdirTemp("", "coach-eval-fault-build")
	Expect(err).NotTo(HaveOccurred())
	DeferCleanup(os.RemoveAll, directory)
	binary := filepath.Join(directory, "coach-eval")
	build := exec.Command("go", "build", "-tags", "coach_eval_fault", "-o", binary, "./cmd/coach-eval")
	build.Dir = filepath.Join("..", "..")
	output, err := build.CombinedOutput()
	Expect(err).NotTo(HaveOccurred(), string(output))
	return binary
}

var validationSteps = []string{"native", "delete_baseline", "layer_graph", "gocognit"}

var _ = Describe("coach-eval generate rejection and replacement", Ordered, ContinueOnFailure, func() {
	var faultBinary string

	BeforeAll(func() {
		faultBinary = buildCoachEvalWithFaultSeam()
	})

	faulty := func(fault string, args ...string) generateOutcome {
		return runGenerateBinary(faultBinary, GinkgoT().TempDir(), []string{faultVariable + "=" + fault}, args...)
	}

	DescribeTable("replaces a program that fails a validation step with the next seed in the same cell (AC-14)",
		func(step string) {
			outcome := faulty(step+":5", "--cell", "0", "--seed", "5")
			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			replaced := decodeGenerated(outcome.stdout)

			Expect(replaced.Seed).To(Equal(int64(6)))
			Expect(replaced.Replacements).To(Equal(1))
			if step == "native" {
				next := generateOne(GinkgoT().TempDir(), "--cell", "0", "--seed", "6")
				Expect(replaced.ProgramID).To(Equal(next.ProgramID), "the replacement is the program the next seed generates")
			}
		},
		Entry("native checks", "native"),
		Entry("delete baseline", "delete_baseline"),
		Entry("layer graph grader", "layer_graph"),
		Entry("gocognit grader", "gocognit"),
		Entry("follow-on stub", "followon_stub"),
		Entry("follow-on delete baseline", "followon_delete_baseline"),
		Entry("follow-on oracle", "followon_oracle"),
		Entry("G2 on the oracles", "g2"),
		Entry("native G1 on the oracles", "g1_native"),
	)

	It("exits 0 at a rejection rate of exactly 50% (AC-15)", func() {
		outcome := faulty("native:5", "--cell", "0", "--seed", "5")

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		Expect(decodeGenerated(outcome.stdout).Replacements).To(Equal(1), "one rejection in two attempts")
	})

	DescribeTable("exits 8 above 50%, prints no program, and names the step and the operators (AC-15)",
		func(step string) {
			outcome := faulty(step+":5,6", "--cell", "0", "--seed", "5")

			Expect(outcome.exitCode).To(Equal(8), outcome.stderr)
			Expect(outcome.stdout).To(BeEmpty())
			Expect(outcome.stderr).To(ContainSubstring("rejection rate"))
			Expect(outcome.stderr).To(ContainSubstring("2 of 3"))
			Expect(outcome.stderr).To(ContainSubstring("67%"), "two of three, rounded and not truncated")
			Expect(outcome.stderr).To(ContainSubstring(step))
			Expect(outcome.stderr).To(ContainSubstring("operators"))
		},
		Entry("native checks", "native"),
		Entry("delete baseline", "delete_baseline"),
		Entry("layer graph grader", "layer_graph"),
		Entry("gocognit grader", "gocognit"),
		Entry("follow-on stub", "followon_stub"),
		Entry("follow-on delete baseline", "followon_delete_baseline"),
		Entry("follow-on oracle", "followon_oracle"),
		Entry("G2 on the oracles", "g2"),
		Entry("native G1 on the oracles", "g1_native"),
	)

	It("gives up after ten attempts in one cell and exits 8 (AC-15)", func() {
		seeds := []string{}
		for seed := 5; seed < 20; seed++ {
			seeds = append(seeds, fmt.Sprint(seed))
		}
		outcome := faulty("native:"+strings.Join(seeds, ","), "--cell", "0", "--seed", "5")

		Expect(outcome.exitCode).To(Equal(8), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring("10 of 10"))
	})

	It("exits 8 when one cell of a run is rejected at every attempt, although the whole run stays under 50% (AC-15)", func() {
		lastTuningCell := "1661"
		outcome := faulty("native:0,1,2,3,4,5,6,7,8,9@"+lastTuningCell, "--tuning")

		Expect(outcome.exitCode).To(Equal(8), outcome.stderr)
		Expect(outcome.stdout).To(BeEmpty(), "no program is printed for a run that lost a cell")
		Expect(outcome.stderr).To(ContainSubstring("10 of 21"), "eleven first-try cells and ten rejections")
		Expect(outcome.stderr).To(ContainSubstring("every attempt"))
	})

	It("applies a fault without a cell to every cell, not only the first (AC-14)", func() {
		outcome := faulty("native:5", "--cell", "100", "--seed", "5")

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		Expect(decodeGenerated(outcome.stdout).Replacements).To(Equal(1))
	})

	It("is not reachable from the release build, which ignores the fault variable (AC-16)", func() {
		outcome := runGenerate(GinkgoT().TempDir(), []string{faultVariable + "=native:5"}, "--cell", "0", "--seed", "5")
		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)

		program := decodeGenerated(outcome.stdout)
		Expect(program.Seed).To(Equal(int64(5)))
		Expect(program.Replacements).To(Equal(0))
	})

	It("rejects a fault that names no validation step (AC-16)", func() {
		outcome := faulty("no_such_step:5", "--cell", "0", "--seed", "5")

		Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
		Expect(outcome.stderr).To(ContainSubstring("no_such_step"))
	})
})
