package antipatternfixtures_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgen/antipatternfixtures"
)

// brokenTemplate is a template whose good variant fails go vet.
func brokenTemplate() antipatternfixtures.Template {
	broken := antipatternfixtures.Templates()[0]
	broken.ID = "broken-template"
	good := broken.Good
	broken.Good = func(value int) map[string]string { return withUnusedVariable(good(value)) }
	return broken
}

// withUnusedVariable is files with a function that go vet refuses added to
// each Go file.
func withUnusedVariable(files map[string]string) map[string]string {
	broken := map[string]string{}
	for name, content := range files {
		broken[name] = content
		if strings.HasSuffix(name, ".go") {
			broken[name] += "\nfunc unused() { x := 1 }\n"
		}
	}
	return broken
}

var _ = Describe("refusing a profile or a fixture (#872 Task 5, AC-4.5)", func() {
	var root, sibling, kept string
	BeforeEach(func() {
		parent := GinkgoT().TempDir()
		root = filepath.Join(parent, "derived")
		sibling = filepath.Join(parent, "sibling.txt")
		kept = filepath.Join(root, "other-label", "kept.txt")
		Expect(os.MkdirAll(filepath.Dir(kept), 0o755)).To(Succeed())
		Expect(os.WriteFile(kept, []byte("kept\n"), 0o644)).To(Succeed())
		Expect(os.WriteFile(sibling, []byte("kept\n"), 0o644)).To(Succeed())
	})

	expectRootAndSiblingKept := func() {
		GinkgoHelper()
		Expect(sibling).To(BeAnExistingFile(), "a sibling of the output root survives")
		Expect(kept).To(BeAnExistingFile(), "the output root and another label's fixtures survive")
	}

	DescribeTable("refuses with exit 2 before removing anything, leaving the output root and its siblings as they were",
		func(raw []byte, validate antipatternfixtures.Validator, label, reason string) {
			_, err := antipatternfixtures.Generate(raw, validate, root, label, antipatternfixtures.Templates(), antipatternfixtures.GoVet)

			Expect(exitOf(err)).To(Equal(2))
			Expect(err.Error()).To(ContainSubstring(reason))
			expectRootAndSiblingKept()
		},
		Entry("a profile the schema validator rejects", everyShape(), antipatternfixtures.Validator(func([]byte) error { return errors.New("closed schema: unexpected field") }),
			"secret-label", "closed schema: unexpected field"),
		Entry("a profile from another extractor version", []byte(strings.Replace(string(everyShape()), `"0.2.0"`, `"0.1.0"`, 1)), antipatternfixtures.Validator(acceptAll),
			"secret-label", `extractor_version "0.1.0"`),
		Entry("a profile of another schema version", []byte(strings.Replace(string(everyShape()), `"schema_version":"1"`, `"schema_version":"2"`, 1)), antipatternfixtures.Validator(acceptAll),
			"secret-label", `schema_version "2"`),
		Entry("the label .., even past a validator that accepts it", everyShape(), antipatternfixtures.Validator(acceptAll), "..", `label ".."`),
		Entry("the label .", everyShape(), antipatternfixtures.Validator(acceptAll), ".", `label "."`),
		Entry("an empty label", everyShape(), antipatternfixtures.Validator(acceptAll), "", `label ""`),
		Entry("a label with a slash", everyShape(), antipatternfixtures.Validator(acceptAll), "other-label/x", `label "other-label/x"`),
		Entry("a label with a backslash", everyShape(), antipatternfixtures.Validator(acceptAll), `a\b`, `label "a\\b"`),
	)

	When("a variant does not pass go vet", func() {
		It("leaves that fixture out of the directory and the result, writes the others, and exits 3 naming it (AC-4.5)", func() {
			templates := append(antipatternfixtures.Templates(), brokenTemplate())

			result, err := antipatternfixtures.Generate(everyShape(), acceptAll, root, "secret-label", templates, antipatternfixtures.GoVet)

			Expect(exitOf(err)).To(Equal(3))
			Expect(err.Error()).To(ContainSubstring("broken-template"))
			out := filepath.Join(root, "secret-label")
			Expect(filepath.Join(out, "broken-template")).NotTo(BeADirectory())
			Expect(ids(out)).To(ContainElement("arrow-code"))
			Expect(result.Emitted).NotTo(ContainElement(HaveField("ID", "broken-template")))
			expectRootAndSiblingKept()
		})
	})
})
