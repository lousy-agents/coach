package main_test

import (
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const gremlinsOverrideVariable = "COACH_EVAL_GREMLINS"

// standInGremlins writes the report a chosen mutation run would: it logs each invocation, takes the report's path
// from --output, and names the entry package's source file so the mutants fall inside the api surface's route.
const standInGremlins = `echo run >> "$GREMLINS_LOG"
out=""
while [ $# -gt 0 ]; do
  if [ "$1" = "--output" ]; then out="$2"; fi
  shift
done
file=$(grep -rlE 'func (\(Service\) )?(Get|Set)[A-Za-z]*\(|func Run\(' --include=*.go internal | grep -v _test.go | head -1)
mode="$STANDIN_MODE"
if [ "$mode" = "bad-once" ]; then
  if [ -e "$GREMLINS_STATE" ]; then mode=good; else : > "$GREMLINS_STATE"; mode=bad; fi
fi
good='{"type":"A","status":"KILLED","line":1,"column":1},{"type":"A","status":"KILLED","line":2,"column":1},{"type":"A","status":"KILLED","line":3,"column":1},{"type":"A","status":"KILLED","line":4,"column":1},{"type":"A","status":"KILLED","line":5,"column":1}'
bad='{"type":"A","status":"KILLED","line":1,"column":1},{"type":"A","status":"KILLED","line":2,"column":1},{"type":"A","status":"KILLED","line":3,"column":1},{"type":"A","status":"LIVED","line":4,"column":1},{"type":"A","status":"LIVED","line":5,"column":1}'
if [ "$mode" = "bad" ]; then mutations="$bad"; else mutations="$good"; fi
printf '{"files":[{"file_name":"%s","mutations":[%s]}]}' "$file" "$mutations" > "$out"
`

type standIn struct {
	binary, log, state string
}

func newStandIn() standIn {
	directory := GinkgoT().TempDir()
	return standIn{
		binary: writeExecutable(directory, "gremlins", standInGremlins),
		log:    filepath.Join(directory, "invocations.log"),
		state:  filepath.Join(directory, "state"),
	}
}

func (s standIn) environment(mode string) []string {
	return []string{gremlinsOverrideVariable + "=" + s.binary, "GREMLINS_LOG=" + s.log, "GREMLINS_STATE=" + s.state, "STANDIN_MODE=" + mode, "COACH_BIN="}
}

func (s standIn) invocations() int {
	content, err := os.ReadFile(s.log)
	if os.IsNotExist(err) {
		return 0
	}
	Expect(err).NotTo(HaveOccurred())
	return len(strings.Fields(string(content)))
}

func validationOf(program generatedProgram) map[string]any {
	return manifestOf(program)["validation"].(map[string]any)
}

var _ = Describe("coach-eval generate and the mutation gate", Ordered, ContinueOnFailure, func() {
	var faultBinary string

	BeforeAll(func() {
		faultBinary = buildCoachEvalWithFaultSeam()
	})

	gated := func(cacheHome string, standIn standIn, mode string, args ...string) generateOutcome {
		return runGenerateBinary(faultBinary, cacheHome, standIn.environment(mode), args...)
	}

	When("a program's mutants are all detected", func() {
		It("records the score and the share it judged for both suites and emits the program (AC-24)", func() {
			gremlins := newStandIn()

			outcome := gated(GinkgoT().TempDir(), gremlins, "good", "--cell", "1", "--seed", "3")
			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			validation := validationOf(decodeGenerated(outcome.stdout))

			Expect(validation["mutation_checked"]).To(Equal(true))
			Expect(validation["mutation_score"]).To(BeEquivalentTo(1))
			Expect(validation["not_covered_share"]).To(BeEquivalentTo(0))
			runs := validation["mutation_runs"].(map[string]any)
			Expect(runs["pass_to_pass"].(map[string]any)["valid"]).To(BeEquivalentTo(5))
			Expect(runs["fail_to_pass"].(map[string]any)["valid"]).To(BeEquivalentTo(5))
			Expect(gremlins.invocations()).To(Equal(2), "one run on the base, one on the first feature's oracle")
		})
	})

	When("a program's mutants are not detected well enough", func() {
		It("replaces it with the next seed and counts the rejection at the mutation step (AC-24)", func() {
			gremlins := newStandIn()

			outcome := gated(GinkgoT().TempDir(), gremlins, "bad-once", "--cell", "1", "--seed", "3")
			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			program := decodeGenerated(outcome.stdout)

			Expect(program.Replacements).To(Equal(1))
			Expect(program.Seed).To(Equal(int64(4)))
		})

		It("exits 8 when no program passes, and names the mutation step (AC-24)", func() {
			outcome := gated(GinkgoT().TempDir(), newStandIn(), "bad", "--cell", "1", "--seed", "3")

			Expect(outcome.exitCode).To(Equal(8), outcome.stderr)
			Expect(outcome.stdout).To(BeEmpty())
			Expect(outcome.stderr).To(ContainSubstring("mutation"))
		})
	})

	When("a program with the same content is generated again under another coach binary", func() {
		It("does not run Gremlins again: the result is cached by the program's content and the Gremlins version (AC-26)", func() {
			gremlins := newStandIn()
			directory := GinkgoT().TempDir()
			cacheHome := GinkgoT().TempDir()
			three := writeExecutable(directory, "coach-three", `printf '{"signals":[]}'`)
			five := writeExecutable(directory, "coach-five", `printf '{"signals":[{}]}'`)
			args := []string{"--cell", "1", "--seed", "3"}

			first := runGenerateBinary(faultBinary, cacheHome, append(gremlins.environment("good"), "COACH_BIN="+three), args...)
			Expect(first.exitCode).To(Equal(0), first.stderr)
			Expect(gremlins.invocations()).To(Equal(2))
			second := runGenerateBinary(faultBinary, cacheHome, append(gremlins.environment("good"), "COACH_BIN="+five), args...)
			Expect(second.exitCode).To(Equal(0), second.stderr)

			Expect(decodeGenerated(second.stdout).Cache).To(Equal("miss"), "another coach binary is another program record")
			Expect(gremlins.invocations()).To(Equal(2), "the mutation result of the same program is reused")
		})
	})

	When("--no-mutation is given", func() {
		It("skips the gate, records that it was skipped, and never starts Gremlins (AC-27)", func() {
			gremlins := newStandIn()

			outcome := gated(GinkgoT().TempDir(), gremlins, "bad", "--cell", "1", "--seed", "3", "--no-mutation")
			Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
			validation := validationOf(decodeGenerated(outcome.stdout))

			Expect(validation["mutation_checked"]).To(Equal(false))
			Expect(validation).NotTo(HaveKey("mutation_score"))
			Expect(gremlins.invocations()).To(Equal(0))
		})

		It("is refused for the held-out panel, which is never generated unchecked (AC-27)", func() {
			outcome := gated(GinkgoT().TempDir(), newStandIn(), "good", "--cell", "1", "--seed", "3", "--panel", "held_out", "--no-mutation")

			Expect(outcome.exitCode).To(Equal(2), outcome.stderr)
			Expect(outcome.stdout).To(BeEmpty())
			Expect(outcome.stderr).To(ContainSubstring("--no-mutation"))
			Expect(outcome.stderr).To(ContainSubstring("held_out"))
		})
	})

	It("runs the gate by default with the pinned tool, and ignores the test-only override in the release build (AC-25)", func() {
		gremlins := newStandIn()

		outcome := runGenerate(GinkgoT().TempDir(), gremlins.environment("bad"), "--cell", "1", "--seed", "3")
		Expect(outcome.exitCode).To(Equal(0), outcome.stderr)
		validation := validationOf(decodeGenerated(outcome.stdout))

		Expect(gremlins.invocations()).To(Equal(0), "the release build does not read the override")
		Expect(validation["mutation_checked"]).To(Equal(true))
		Expect(validation["mutation_score"]).To(BeNil(), "today's programs are branch-free: Gremlins finds no mutant, so the gate passes vacuously")
		runs := validation["mutation_runs"].(map[string]any)
		Expect(runs["pass_to_pass"].(map[string]any)["valid"]).To(BeEquivalentTo(0))
		Expect(runs["fail_to_pass"].(map[string]any)["valid"]).To(BeEquivalentTo(0))
	})
})
