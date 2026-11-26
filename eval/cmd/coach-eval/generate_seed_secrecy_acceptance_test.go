package main_test

import (
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach-eval generate for the held-out panel", func() {
	It("never prints the seed it generated with, which the protocol keeps only as a hash (AC-18)", func() {
		outcome := runGenerate(GinkgoT().TempDir(), []string{"EVAL_ORACLE_DIR=" + GinkgoT().TempDir()},
			"--layers", "5", "--packages", "4", "--entry", "command", "--depth", "5", "--field", "nested", "--direction", "write", "--trap", "complexity",
			"--seed", "424242", "--panel", "held_out")
		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)

		fields := map[string]any{}
		Expect(json.Unmarshal([]byte(strings.TrimSpace(outcome.stdout)), &fields)).To(Succeed())
		Expect(fields).NotTo(HaveKey("seed"))
		Expect(outcome.stdout).NotTo(ContainSubstring("424242"))
	})
})
