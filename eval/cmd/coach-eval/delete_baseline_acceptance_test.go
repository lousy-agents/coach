package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgen"
)

// The hidden test below passes only while the entry file is absent, so the stub fails it (the do-nothing
// baseline is rejected) and a tree with the entry deleted passes it. Only the delete baseline can catch it.
const passesWhenTheEntryIsDeleted = `package feature_test

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEntryFileIsGone(t *testing.T) {
	matches, _ := filepath.Glob("../../internal/*/*/*.go")
	for _, path := range matches {
		if b, err := os.ReadFile(path); err == nil && containsEntry(string(b)) {
			t.Fatalf("entry present in %s", path)
		}
	}
}

func containsEntry(source string) bool {
	return len(source) > 0 && (contains(source, "func Get") || contains(source, "func Set") || contains(source, "func Run") || contains(source, "func (Service)"))
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
`

var _ = Describe("native validation of a generated program", func() {
	It("rejects a hidden test that passes once the entry is deleted, naming the delete baseline (AC-14)", func() {
		cell, err := evalgen.CellFromIndex(0)
		Expect(err).NotTo(HaveOccurred())
		program, err := evalgen.Generate(cell, 1)
		Expect(err).NotTo(HaveOccurred())
		program.Tests = map[string][]byte{"tests/feature/feature_test.go": []byte(passesWhenTheEntryIsDeleted)}

		err = evalgen.Validate(program)

		Expect(err).To(MatchError(ContainSubstring("delete baseline")))
	})
})
