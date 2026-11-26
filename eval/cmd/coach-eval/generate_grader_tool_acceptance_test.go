package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach-eval generate when a grader cannot run", func() {
	It("exits 2 and names the grader, because the graded detectors run on every generated base (AC-17)", func() {
		outcome := runGenerate(GinkgoT().TempDir(), []string{"EVAL_MODULE_DIR=" + GinkgoT().TempDir()}, "--cell", "0", "--seed", "5")

		Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
		Expect(outcome.stdout).To(BeEmpty())
		Expect(outcome.stderr).To(ContainSubstring("gocognit"))
	})
})
