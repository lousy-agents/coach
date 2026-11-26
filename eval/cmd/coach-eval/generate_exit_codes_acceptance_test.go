package main_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach-eval generate exit codes", func() {
	var cacheHome string

	BeforeEach(func() {
		cacheHome = GinkgoT().TempDir()
	})

	DescribeTable("exits 2 and names the input when the input is missing or invalid (AC-9)",
		func(args []string, extraEnv []string, mentions string) {
			outcome := runGenerate(cacheHome, extraEnv, args...)

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stdout).To(BeEmpty())
			Expect(outcome.stderr).To(ContainSubstring(mentions))
		},
		Entry("a cell index past the last cell", []string{"--cell", "1728"}, nil, "cell"),
		Entry("a layer count outside 3 to 5", []string{"--layers", "6", "--packages", "2", "--entry", "function", "--depth", "2", "--field", "scalar", "--direction", "read", "--trap", "layering"}, nil, "layers"),
		Entry("an entry kind that is not one of the four", []string{"--layers", "3", "--packages", "2", "--entry", "lambda", "--depth", "2", "--field", "scalar", "--direction", "read", "--trap", "layering"}, nil, "entry"),
		Entry("an unknown flag", []string{"--no-such-flag"}, nil, "no-such-flag"),
		Entry("no cell at all", []string{"--seed", "1"}, nil, "cell"),
		Entry("the held-out panel without EVAL_ORACLE_DIR", []string{"--cell", "1", "--panel", "held_out"}, []string{"EVAL_ORACLE_DIR="}, "EVAL_ORACLE_DIR"),
		Entry("a PATH with no go on it", []string{"--cell", "0"}, []string{"PATH=" + os.TempDir() + "/no-such-directory"}, "go"),
	)
})
