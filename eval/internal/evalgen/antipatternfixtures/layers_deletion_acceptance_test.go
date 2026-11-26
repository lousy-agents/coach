package antipatternfixtures_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/lousy-agents/coach/eval/internal/evalgen/antipatternfixtures"
)

// withUnvettableGood is the template with its good variant made to fail go vet.
func withUnvettableGood(template antipatternfixtures.Template) antipatternfixtures.Template {
	good := template.Good
	template.Good = func(value int) map[string]string { return withUnusedVariable(good(value)) }
	return template
}

// withBrokenSkippedLayer is the templates with the skipped-layer fixture's good variant made to fail go vet.
func withBrokenSkippedLayer() []antipatternfixtures.Template {
	var templates []antipatternfixtures.Template
	for _, template := range antipatternfixtures.Templates() {
		if template.ID == "skipped-layer" {
			template = withUnvettableGood(template)
		}
		templates = append(templates, template)
	}
	return templates
}

var _ = Describe("the deletion-safety guards on a layer fixture, whose policy sits in a nested dot directory (#900)", func() {
	var root, sibling, kept string
	BeforeEach(func() {
		parent := GinkgoT().TempDir()
		root = filepath.Join(parent, "derived")
		sibling = filepath.Join(parent, "sibling.txt")
		kept = filepath.Join(root, "other-label", "fixture", "bad", ".lousy-agents", "coach.json.txt")
		Expect(os.MkdirAll(filepath.Dir(kept), 0o755)).To(Succeed())
		Expect(os.WriteFile(kept, []byte("kept\n"), 0o644)).To(Succeed())
		Expect(os.WriteFile(sibling, []byte("kept\n"), 0o644)).To(Succeed())
	})

	expectRootAndSiblingKept := func() {
		GinkgoHelper()
		Expect(sibling).To(BeAnExistingFile())
		Expect(kept).To(BeAnExistingFile(), "another label's fixtures survive, nested policy included")
	}

	It("clears only its own label on a rerun, nested policy files included, and keeps another label's", func() {
		raw := layered(map[string]int{"4": 1}, evenly(4, map[string]int{"1": 1}))
		_, err := antipatternfixtures.Generate(raw, acceptAll, root, "secret-label", antipatternfixtures.Templates(), antipatternfixtures.GoVet)
		Expect(err).NotTo(HaveOccurred())
		first := filepath.Join(root, "secret-label", "skipped-layer", "bad", ".lousy-agents", "coach.json.txt")
		Expect(first).To(BeAnExistingFile())

		_, err = antipatternfixtures.Generate(profileWith(map[string]int{"4": 1}, map[string]int{}, map[string]int{}, 0), acceptAll, root, "secret-label", antipatternfixtures.Templates(), antipatternfixtures.GoVet)

		Expect(err).NotTo(HaveOccurred())
		Expect(first).NotTo(BeAnExistingFile(), "a layer fixture no longer observed leaves nothing behind, nested policy included")
		Expect(filepath.Join(root, "secret-label", "skipped-layer")).NotTo(BeADirectory())
		expectRootAndSiblingKept()
	})

	DescribeTable("refuses a layered profile with exit 2 before removing anything",
		func(label string) {
			raw := layered(map[string]int{"4": 1}, evenly(4, map[string]int{"1": 1}))

			_, err := antipatternfixtures.Generate(raw, acceptAll, root, label, antipatternfixtures.Templates(), antipatternfixtures.GoVet)

			Expect(exitOf(err)).To(Equal(2))
			expectRootAndSiblingKept()
		},
		Entry("the label ..", ".."), Entry("the label .", "."), Entry("an empty label", ""), Entry("a label with a slash", "other-label/fixture"),
	)

	It("leaves a layer fixture whose variant fails go vet out, writes the others, and exits 3 naming it", func() {
		raw := layered(map[string]int{"3": 1}, evenly(3, map[string]int{"1": 1}))

		result, err := antipatternfixtures.Generate(raw, acceptAll, root, "secret-label", withBrokenSkippedLayer(), antipatternfixtures.GoVet)

		Expect(exitOf(err)).To(Equal(3))
		Expect(err.Error()).To(ContainSubstring("skipped-layer"))
		Expect(filepath.Join(root, "secret-label", "skipped-layer")).NotTo(BeADirectory())
		Expect(ids(filepath.Join(root, "secret-label"))).To(Equal([]string{"dependency-rule-breach"}))
		Expect(result.Emitted).To(HaveLen(1))
		expectRootAndSiblingKept()
	})
})
