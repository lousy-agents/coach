package evalpatterns_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// antipatternVariant copies one variant of a #871 fixture into a fresh
// directory with the .txt suffixes the fixtures carry (so the repository's
// own scan never sees them) removed, as a stand-in checkout.
func antipatternVariant(fixture, variant string) string {
	GinkgoHelper()
	source := filepath.Join("..", "..", "fixtures", "antipatterns", fixture, variant)
	root := filepath.Join(GinkgoT().TempDir(), "variant")
	Expect(filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(source, path)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		writeFile(filepath.Join(root, strings.TrimSuffix(rel, ".txt")), string(raw))
		return nil
	})).To(Succeed())
	return root
}

// largestBucket is the largest value a distribution in the profile counts.
func largestBucket(profile map[string]any, group, name string) int {
	GinkgoHelper()
	largest := -1
	for bucket := range measure(profile, group, name).(map[string]any) {
		value, err := strconv.Atoi(bucket)
		Expect(err).NotTo(HaveOccurred())
		largest = max(largest, value)
	}
	return largest
}

func variantProfile(fixture, variant string) map[string]any {
	GinkgoHelper()
	return decodeProfile(extract(antipatternVariant(fixture, variant)))
}

var _ = Describe("the measures over #871's anti-pattern fixtures as stand-in references (#872 Task 2)", func() {
	DescribeTable("put the bad variant over Coach's threshold and the good variant under it",
		func(fixture, shape string, threshold int) {
			Expect(largestBucket(variantProfile(fixture, "bad"), "shapes", shape)).To(BeNumerically(">=", threshold))
			Expect(largestBucket(variantProfile(fixture, "good"), "shapes", shape)).To(BeNumerically("<", threshold))
		},
		Entry("arrow code: nesting depth 4 or more", "arrow-code", "nesting", 4),
		Entry("a spaghetti conditional: cognitive complexity over 15", "spaghetti-conditional", "cognitive", 16),
		Entry("a god file: 12 or more branch points in the file", "god-file", "branch_density", 12),
	)

	It("counts the bad variant's constructor as doing work and the good one's as assigning only", func() {
		Expect(measure(variantProfile("constructor-does-real-work", "bad"), "shapes", "constructor")).To(Equal(map[string]any{"assigns_only": 0.0, "does_work": 1.0}))
		Expect(measure(variantProfile("constructor-does-real-work", "good"), "shapes", "constructor")).To(Equal(map[string]any{"assigns_only": 1.0, "does_work": 0.0}))
	})

	It("counts more methods on the god struct than on its split", func() {
		bad := measure(variantProfile("god-struct", "bad"), "dimensions", "entry").(map[string]any)["method"]
		good := measure(variantProfile("god-struct", "good"), "dimensions", "entry").(map[string]any)["method"]

		Expect(bad).To(BeNumerically(">", good))
	})

	It("writes none of a fixture's package or function names into its profile (AC-1.3)", func() {
		raw := string(profileJSON(variantProfile("arrow-code", "bad")))

		for _, name := range []string{"ledger", "apply", "Apply", "arrow"} {
			Expect(raw).NotTo(ContainSubstring(name))
		}
	})
})
