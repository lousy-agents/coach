package main_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach-eval generate --language typescript, with EVAL_TSC naming a directory (781b)", Label(typeScriptLabel), func() {
	cell := typeScriptCellFlags(programCells[0])

	It("uses the typescript package directory as the compiler it holds", func() {
		_ = installedTSC()
		packageDir := filepath.Join(repositoryRoot(), "js", "semantics", "node_modules", "typescript")

		outcome := runGenerate(GinkgoT().TempDir(), []string{"EVAL_TSC=" + packageDir}, cell...)

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
	})

	It("says it expected the tsc entry point when the directory holds none (exit 2)", func() {
		outcome := runGenerate(GinkgoT().TempDir(), []string{"EVAL_TSC=" + GinkgoT().TempDir()}, cell...)

		Expect(outcome.exitCode).To(Equal(2))
		Expect(outcome.stderr).To(ContainSubstring("expected the tsc entry point"))
	})
})
