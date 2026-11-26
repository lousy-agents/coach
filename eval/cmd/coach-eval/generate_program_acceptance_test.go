package main_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var programCells = []programCell{
	{3, 2, "function", 2, "scalar", "read", "layering"},
	{4, 3, "method", 3, "collection", "write", "complexity"},
	{5, 4, "http_handler", 5, "nested", "read", "layering"},
	{3, 4, "command", 4, "collection", "read", "layering"},
	{5, 2, "http_handler", 3, "scalar", "write", "layering"},
	{4, 2, "command", 5, "nested", "write", "complexity"},
	{3, 2, "method", 5, "nested", "read", "layering"},
	{5, 3, "function", 2, "nested", "write", "layering"},
}

var _ = Describe("a generated Go program", func() {
	for _, cell := range programCells {
		It("compiles with the stub, fails only its feature tests, and passes them with the oracle ("+strings.Join(cell.flags(), " ")+") (AC-10)", func() {
			program := generateOne(GinkgoT().TempDir(), append(cell.flags(), "--seed", "9")...)
			work := GinkgoT().TempDir()
			copyTree(readTree(program.Path), work)

			Expect(goIn(work, "build", "./...").exitCode).To(Equal(0))
			Expect(goIn(work, "vet", "./...").exitCode).To(Equal(0))
			visible := goIn(work, "test", "./...")
			Expect(visible.exitCode).To(Equal(0), "the visible PASS_TO_PASS tests pass on the stub: "+visible.stdout+visible.stderr)
			Expect(readTree(program.Path)).NotTo(HaveKey(MatchRegexp(`^tests/feature/`)), "the hidden FAIL_TO_PASS tests are not in the tree")

			hidden := readTree(filepath.Join(program.OracleDir, "tests"))
			Expect(hidden).NotTo(BeEmpty())
			copyTree(hidden, work)
			stubRun := goIn(work, "test", "./...")
			Expect(stubRun.exitCode).NotTo(Equal(0), "the FAIL_TO_PASS tests fail on the stub")
			Expect(stubRun.stdout).To(MatchRegexp(`(?m)^FAIL\s+\S+/tests/feature\b`))
			Expect(stubRun.stdout).NotTo(MatchRegexp(`(?m)^FAIL\s+\S+/(layer|api|service|store)\S*`), "no other package fails")

			solution := readTree(filepath.Join(program.OracleDir, "solution"))
			Expect(solution).NotTo(BeEmpty())
			copyTree(solution, work)
			oracleRun := goIn(work, "test", "./...")
			Expect(oracleRun.exitCode).To(Equal(0), "the clean oracle passes every test: "+oracleRun.stdout+oracleRun.stderr)
			Expect(goIn(work, "vet", "./...").exitCode).To(Equal(0))
		})
	}

	It("keeps the oracle out of the tree and the oracle directory out of the repository (AC-10)", func() {
		cacheHome := GinkgoT().TempDir()
		program := generateOne(cacheHome, append(programCells[0].flags(), "--seed", "9")...)

		Expect(program.OracleDir).To(HavePrefix(cacheHome))
		_, err := os.Stat(filepath.Join(program.Path, "solution"))
		Expect(os.IsNotExist(err)).To(BeTrue())
	})
})
