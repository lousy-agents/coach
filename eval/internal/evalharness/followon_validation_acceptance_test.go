package evalharness_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// passesWhenTheEntryIsDeleted fails while an entry function is declared anywhere under internal and passes once
// none is, so only a delete baseline can reject it.
const followOnPassesWhenTheEntryIsDeleted = `package followon_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEntryIsGone(t *testing.T) {
	matches, _ := filepath.Glob("../../internal/*/*/*.go")
	for _, path := range matches {
		source, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(source), "func Describe") && (strings.Contains(string(source), "func Get") || strings.Contains(string(source), "func Set") || strings.Contains(string(source), "func Run") || strings.Contains(string(source), "func (Service)")) {
			t.Fatalf("entry present in %s", path)
		}
	}
}
`

const followOnPassesAnywhere = `package followon_test

import "testing"

func TestNothing(t *testing.T) {}
`

func followOnProgram() evalgen.Program {
	cell, err := evalgen.CellFromIndex(1)
	Expect(err).NotTo(HaveOccurred())
	program, err := evalgen.Generate(cell, 1)
	Expect(err).NotTo(HaveOccurred())
	return program
}

var _ = Describe("native validation of a program's follow-on slot", func() {
	It("rejects follow-on tests that already pass on the first feature's oracle, at the follow-on stub step (AC-20)", func() {
		program := followOnProgram()
		program.FollowOnTests = map[string][]byte{"tests/followon/followon_test.go": []byte(followOnPassesAnywhere)}

		err := evalgen.Validate(program)

		Expect(err).To(MatchError(ContainSubstring("follow-on stub")))
		Expect(rejectionStep(err)).To(Equal(evalgen.StepFollowOnStub))
	})

	It("rejects follow-on tests that pass once the entry is deleted, at the follow-on delete baseline (AC-20)", func() {
		program := followOnProgram()
		program.FollowOnTests = map[string][]byte{"tests/followon/followon_test.go": []byte(followOnPassesWhenTheEntryIsDeleted)}

		err := evalgen.Validate(program)

		Expect(err).To(MatchError(ContainSubstring("follow-on delete baseline")))
		Expect(rejectionStep(err)).To(Equal(evalgen.StepFollowOnDeleteBaseline))
	})

	It("rejects a follow-on oracle that does not pass its own tests, at the follow-on oracle step (AC-20)", func() {
		program := followOnProgram()
		program.FollowOnSolution = map[string][]byte{}

		err := evalgen.Validate(program)

		Expect(err).To(MatchError(ContainSubstring("follow-on oracle")))
		Expect(rejectionStep(err)).To(Equal(evalgen.StepFollowOnOracle))
	})

	It("accepts a generated program (AC-20)", func() {
		Expect(evalgen.Validate(followOnProgram())).To(Succeed())
	})
})
