package main_test

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func writeExecutable(directory, name, script string) string {
	path := filepath.Join(directory, name)
	Expect(os.WriteFile(path, []byte("#!/bin/sh\n"+script), 0o755)).To(Succeed())
	return path
}

// pathWithOnlyGo is a PATH on which `go` resolves and `coach` does not.
func pathWithOnlyGo() string {
	goBinary, err := exec.LookPath("go")
	Expect(err).NotTo(HaveOccurred())
	directory := GinkgoT().TempDir()
	Expect(os.Symlink(goBinary, filepath.Join(directory, "go"))).To(Succeed())
	return directory
}

// strictCoach is a coach that prints the report only when it is run the way the design requires: as
// `codesignal --baseline --format=json` inside a git repository whose HEAD holds the program. Anything else exits 2,
// so a spec that reads its report pins the command line, not only the parsing.
func strictCoach(directory, report string) string {
	return writeExecutable(directory, "coach", `[ "$1" = codesignal ] || exit 2
baseline=no; format=no
for argument in "$@"; do
  [ "$argument" = --baseline ] && baseline=yes
  [ "$argument" = --format=json ] && format=yes
done
[ "$baseline" = yes ] && [ "$format" = yes ] || exit 2
git rev-parse --verify HEAD >/dev/null 2>&1 || exit 2
[ -z "$(git status --porcelain)" ] || exit 2
[ -n "$(git ls-files)" ] || exit 2
printf '%s' '`+report+`'`)
}

func baseFindings(program generatedProgram) (count any, reason any) {
	validation := manifestOf(program)["validation"].(map[string]any)
	return validation["coach_base_fp"], validation["coach_base_fp_reason"]
}

var _ = Describe("coach-eval generate records what Coach reports on the unmutated base", func() {
	var cacheHome string

	BeforeEach(func() {
		cacheHome = GinkgoT().TempDir()
	})

	When("no coach binary is named or on PATH", func() {
		It("records coach_base_fp null with the reason, and still emits the program (AC-22)", func() {
			outcome := runGenerate(cacheHome, []string{"PATH=" + pathWithOnlyGo(), "COACH_BIN="}, "--cell", "1", "--seed", "3")
			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)

			count, reason := baseFindings(decodeGenerated(outcome.stdout))

			Expect(count).To(BeNil())
			Expect(reason).To(Equal("no coach binary"))
		})
	})

	When("COACH_BIN names a coach that reports findings on the base", func() {
		It("records their number and never rejects the program for them (AC-22)", func() {
			coach := strictCoach(GinkgoT().TempDir(), `{"signals":[{"rule_id":"a"},{"rule_id":"b"},{"rule_id":"c"}]}`)

			outcome := runGenerate(cacheHome, []string{"COACH_BIN=" + coach}, "--cell", "1", "--seed", "3")
			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			count, reason := baseFindings(decodeGenerated(outcome.stdout))

			Expect(count).To(BeEquivalentTo(3))
			Expect(reason).To(BeNil())
		})

		It("records zero for a report with no signals (AC-22)", func() {
			coach := strictCoach(GinkgoT().TempDir(), `{"signals":[]}`)

			outcome := runGenerate(cacheHome, []string{"COACH_BIN=" + coach}, "--cell", "1", "--seed", "3")
			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			count, reason := baseFindings(decodeGenerated(outcome.stdout))

			Expect(count).To(BeEquivalentTo(0))
			Expect(reason).To(BeNil())
		})
	})

	When("the coach binary fails", func() {
		It("records coach_base_fp null with the failure as the reason and still emits the program (AC-22)", func() {
			coach := writeExecutable(GinkgoT().TempDir(), "coach", "echo broken >&2\nexit 3")

			outcome := runGenerate(cacheHome, []string{"COACH_BIN=" + coach}, "--cell", "1", "--seed", "3")
			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			count, reason := baseFindings(decodeGenerated(outcome.stdout))

			Expect(count).To(BeNil())
			Expect(reason).To(ContainSubstring("coach failed"))
		})
	})

	When("the coach binary changes between runs", func() {
		It("keys the cache by the binary's content, so each binary gets its own record (AC-23)", func() {
			directory := GinkgoT().TempDir()
			three := writeExecutable(directory, "coach-three", `printf '{"signals":[{},{},{}]}'`)
			five := writeExecutable(directory, "coach-five", `printf '{"signals":[{},{},{},{},{}]}'`)
			args := []string{"--cell", "1", "--seed", "3"}

			first := runGenerate(cacheHome, []string{"COACH_BIN=" + three}, args...)
			second := runGenerate(cacheHome, []string{"COACH_BIN=" + five}, args...)
			third := runGenerate(cacheHome, []string{"COACH_BIN=" + three}, args...)
			Expect([]int{first.exitCode, second.exitCode, third.exitCode}).To(Equal([]int{0, 0, 0}))
			firstProgram, secondProgram, thirdProgram := decodeGenerated(first.stdout), decodeGenerated(second.stdout), decodeGenerated(third.stdout)

			Expect(firstProgram.Cache).To(Equal("miss"))
			Expect(secondProgram.Cache).To(Equal("miss"), "another binary is another key")
			Expect(thirdProgram.Cache).To(Equal("hit"), "the first binary's record is still there")
			secondCount, _ := baseFindings(secondProgram)
			thirdCount, _ := baseFindings(thirdProgram)
			Expect(secondCount).To(BeEquivalentTo(5))
			Expect(thirdCount).To(BeEquivalentTo(3))
		})

		It("keys a run with no binary apart from a run with one (AC-23)", func() {
			coach := writeExecutable(GinkgoT().TempDir(), "coach", `printf '{"signals":[]}'`)
			args := []string{"--cell", "1", "--seed", "3"}

			without := runGenerate(cacheHome, []string{"PATH=" + pathWithOnlyGo(), "COACH_BIN="}, args...)
			with := runGenerate(cacheHome, []string{"COACH_BIN=" + coach}, args...)

			Expect(decodeGenerated(without.stdout).Cache).To(Equal("miss"))
			Expect(decodeGenerated(with.stdout).Cache).To(Equal("miss"))
		})
	})
})
