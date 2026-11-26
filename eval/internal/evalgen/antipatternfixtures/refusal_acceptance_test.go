package antipatternfixtures_test

import (
	"errors"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgen/antipatternfixtures"
)

var _ = Describe("refusing a profile or a fixture (#872 Task 5, AC-4.5)", func() {
	var out string
	BeforeEach(func() { out = filepath.Join(GinkgoT().TempDir(), "secret-label") })

	DescribeTable("refuses a profile it cannot read with exit 2, writing nothing",
		func(raw []byte, validate antipatternfixtures.Validator, reason string) {
			_, err := antipatternfixtures.Generate(raw, validate, out, antipatternfixtures.Templates(), antipatternfixtures.GoVet)

			Expect(exitOf(err)).To(Equal(2))
			Expect(err.Error()).To(ContainSubstring(reason))
			Expect(out).NotTo(BeADirectory())
		},
		Entry("one the schema validator rejects", everyShape(), func([]byte) error { return errors.New("closed schema: unexpected field") },
			"closed schema: unexpected field"),
		Entry("one from another extractor version", []byte(strings.Replace(string(everyShape()), `"0.2.0"`, `"0.1.0"`, 1)), antipatternfixtures.Validator(acceptAll),
			`extractor_version "0.1.0"`),
		Entry("one of another schema version", []byte(strings.Replace(string(everyShape()), `"schema_version":"1"`, `"schema_version":"2"`, 1)), antipatternfixtures.Validator(acceptAll),
			`schema_version "2"`),
	)

	When("a variant does not pass go vet", func() {
		It("leaves that fixture out, writes the others, and exits 3 naming it (AC-4.5)", func() {
			broken := antipatternfixtures.Templates()[0]
			broken.ID = "broken-template"
			good := broken.Good
			broken.Good = func(value int) map[string]string {
				files := good(value)
				for name := range files {
					if strings.HasSuffix(name, ".go") {
						files[name] += "\nfunc unused() { x := 1 }\n"
					}
				}
				return files
			}
			templates := append(antipatternfixtures.Templates(), broken)

			_, err := antipatternfixtures.Generate(everyShape(), acceptAll, out, templates, antipatternfixtures.GoVet)

			Expect(exitOf(err)).To(Equal(3))
			Expect(err.Error()).To(ContainSubstring("broken-template"))
			Expect(filepath.Join(out, "broken-template")).NotTo(BeADirectory())
			Expect(ids(out)).To(ContainElement("arrow-code"))
		})
	})
})
