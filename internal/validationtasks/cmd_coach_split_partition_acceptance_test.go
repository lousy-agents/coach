package validationtasks

import (
	"path/filepath"
	"slices"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"os"
)

var _ = Describe("the partition of cmd/coach's acceptance specs across the two verify legs (#920, #605 Option B)", func() {
	var toml string

	BeforeEach(func() {
		toml = readRepoText("mise.toml")
	})

	It("uses one pattern, once each, as the policy half's focus-file and the rest half's skip-file, so no spec runs in both halves or in neither (S1)", func() {
		focus := flagValues(policyCommand(toml), "ginkgo.focus-file")
		skip := flagValues(restCommands(toml)[1], "ginkgo.skip-file")

		Expect(focus).To(HaveLen(1))
		Expect(skip).To(HaveLen(1))
		Expect(focus[0]).NotTo(BeEmpty())
		Expect(skip[0]).To(Equal(focus[0]))
	})

	It("sees a pattern that differs between the halves (S1)", func() {
		mutant := strings.Replace(toml, "-ginkgo.skip-file=policy_", "-ginkgo.skip-file=policy", 1)
		Expect(mutant).NotTo(Equal(toml))

		Expect(flagValues(restCommands(mutant)[1], "ginkgo.skip-file")).NotTo(Equal(flagValues(policyCommand(mutant), "ginkgo.focus-file")))
	})

	It("runs the plain Test functions of cmd/coach in the rest half only: the policy half names the suite entry point with -run and the rest half names no -run (S2)", func() {
		Expect(strings.Fields(policyCommand(toml))).To(ContainElements("-run", "TestCoachAcceptance"))
		Expect(strings.Fields(restCommands(toml)[1])).NotTo(ContainElement("-run"))
		Expect(strings.Fields(restCommands(toml)[0])).NotTo(ContainElement("-run"))
	})

	It("has one suite entry point in cmd/coach, and it is the one the policy half's -run names, so no Ginkgo spec of the package is outside the policy half's reach (S7)", func() {
		entryPoints := runSpecsEntryPoints(filepath.Join("..", "..", "cmd", "coach"))

		Expect(entryPoints).To(Equal([]string{"TestCoachAcceptance"}))
		fields := strings.Fields(policyCommand(toml))
		Expect(fields[slices.Index(fields, "-run")+1]).To(Equal(entryPoints[0]))
	})

	It("counts a second suite entry point, wherever it is written (S7)", func() {
		dir := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(dir, "a_test.go"), []byte("package x\n\nfunc TestA(t *testing.T) {\n\tRunSpecs(t, \"a\")\n}\n"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, "b_test.go"), []byte("package x\n\nfunc TestB(t *testing.T) {\n\tRegisterFailHandler(Fail)\n\tRunSpecs(t, \"b\")\n}\n"), 0o644)).To(Succeed())

		Expect(runSpecsEntryPoints(dir)).To(Equal([]string{"TestA", "TestB"}))
	})
})
