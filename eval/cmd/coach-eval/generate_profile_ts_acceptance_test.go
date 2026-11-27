package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("coach-eval generate --language typescript --profile (#872 Task 4)", Label("typescript"), func() {
	It("names the profile in each TypeScript manifest, as it does for Go (AC-3.4)", func() {
		raw := committedProfile()

		outcome := runGenerate(GinkgoT().TempDir(), nil, "--language", "typescript", "--profile", writeProfile(raw), "--count", "2", "--seed", "7")

		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		programs := decodeAllGenerated(outcome.stdout)
		Expect(programs).To(HaveLen(2))
		for _, program := range programs {
			Expect(readTree(program.Path)).To(HaveKey("tsconfig.json"))
			Expect(manifestProgram(program)).To(HaveKeyWithValue("profile", profileIdentity(raw)))
		}
	})
})
