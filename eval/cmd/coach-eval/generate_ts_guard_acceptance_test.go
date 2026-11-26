package main_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach-eval generate --language, and the toolchain a TypeScript program needs", func() {
	cell := typeScriptCellFlags(programCells[0])

	It("refuses a language it does not know, naming the flag and the values (exit 2)", func() {
		outcome := runGenerate(GinkgoT().TempDir(), nil, append([]string{"--language", "python"}, programCells[0].flags()...)...)

		Expect(outcome.exitCode).To(Equal(2))
		Expect(outcome.stderr).To(ContainSubstring("language"))
		Expect(outcome.stderr).To(ContainSubstring("go"))
		Expect(outcome.stderr).To(ContainSubstring("typescript"))
	})

	It("generates a Go program when no language is given", func() {
		program := generateOne(GinkgoT().TempDir(), programCells[0].flags()...)

		Expect(readTree(program.Path)).To(HaveKey("go.mod"))
	})

	It("uses the compiler EVAL_TSC names, when it is the pinned version", func() {
		outcome := runGenerate(GinkgoT().TempDir(), []string{"EVAL_TSC=" + installedTSC()}, cell...)

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
	})

	It("refuses an EVAL_TSC that is not the pinned version, naming both versions (exit 2)", func() {
		outcome := runGenerate(GinkgoT().TempDir(), []string{"EVAL_TSC=" + stubTSC("5.9.2")}, cell...)

		Expect(outcome.exitCode).To(Equal(2))
		Expect(outcome.stderr).To(ContainSubstring(pinnedTypeScript))
		Expect(outcome.stderr).To(ContainSubstring("5.9.2"))
	})

	It("refuses an EVAL_TSC that does not exist, naming it, and does not fall back to the installed one (exit 2)", func() {
		missing := filepath.Join(GinkgoT().TempDir(), "no-tsc")

		outcome := runGenerate(GinkgoT().TempDir(), []string{"EVAL_TSC=" + missing}, cell...)

		Expect(outcome.exitCode).To(Equal(2))
		Expect(outcome.stderr).To(ContainSubstring("EVAL_TSC"))
		Expect(outcome.stderr).To(ContainSubstring(missing))
	})

	It("uses an EVAL_TSC given as a path relative to the working directory", func() {
		_ = installedTSC()
		dir := filepath.Join(repositoryRoot(), "js", "semantics")

		outcome := runGenerateIn(dir, GinkgoT().TempDir(), []string{"EVAL_TSC=" + filepath.Join("node_modules", ".bin", "tsc")}, cell...)

		Expect(outcome.exitCode).To(Equal(0), "validation runs in another directory, so the path must be made absolute: "+outcome.stderr)
	})

	It("finds the installed compiler from a subdirectory of the repository", func() {
		outcome := runGenerateIn(filepath.Join(repositoryRoot(), "eval", "cmd"), GinkgoT().TempDir(), nil, cell...)
		_ = installedTSC()

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
	})

	It("names EVAL_TSC and the installed location when neither exists (exit 2)", func() {
		outcome := runGenerateIn(GinkgoT().TempDir(), GinkgoT().TempDir(), nil, cell...)

		Expect(outcome.exitCode).To(Equal(2))
		Expect(outcome.stderr).To(ContainSubstring("EVAL_TSC"))
		Expect(outcome.stderr).To(ContainSubstring("js/semantics/node_modules"))
	})

	It("names node when it is not on PATH, with the compiler found (exit 2)", func() {
		outcome := runGenerate(GinkgoT().TempDir(), []string{"PATH=" + noNodePath(), "EVAL_TSC=" + installedTSC()}, cell...)

		Expect(outcome.exitCode).To(Equal(2))
		Expect(outcome.stderr).To(ContainSubstring("node"))
	})

	It("does not need node or the compiler for a Go program", func() {
		outcome := runGenerate(GinkgoT().TempDir(), []string{"PATH=" + noNodePath(), "EVAL_TSC=" + filepath.Join(GinkgoT().TempDir(), "absent")}, programCells[0].flags()...)

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
	})
})

// noNodePath keeps the Go toolchain and drops every directory that holds node.
func noNodePath() string {
	goDir := filepath.Dir(mustLookPath("go"))
	kept := []string{goDir}
	for _, dir := range filepath.SplitList(os.Getenv("PATH")) {
		if _, err := os.Stat(filepath.Join(dir, "node")); err != nil && dir != goDir {
			kept = append(kept, dir)
		}
	}
	return pathFrom(kept)
}
